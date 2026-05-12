package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/storage"
)

func (s *server) handleListReferenceLaps(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeJSON(w, []interface{}{})
		return
	}
	trackID := r.URL.Query().Get("track_id")
	carCodeStr := r.URL.Query().Get("car_code")
	var carCode int32
	if carCodeStr != "" {
		v, _ := strconv.Atoi(carCodeStr)
		carCode = int32(v)
	}
	refs, err := s.database.ListReferenceLaps(r.Context(), trackID, carCode)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if refs == nil {
		refs = []db.ReferenceLap{}
	}
	writeJSON(w, refs)
}

func (s *server) handleSetReferenceLap(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		http.Error(w, "no database", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		TrackID   string `json:"track_id"`
		CarCode   int32  `json:"car_code"`
		SessionID string `json:"session_id"`
		LapNumber int    `json:"lap_number"`
		TimeMs    int32  `json:"time_ms"`
		S3Key     string `json:"s3_key"`
		Label     string `json:"label"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Label == "" {
		req.Label = "best"
	}
	ref := &db.ReferenceLap{
		TrackID:   req.TrackID,
		CarCode:   req.CarCode,
		SessionID: req.SessionID,
		LapNumber: req.LapNumber,
		TimeMs:    req.TimeMs,
		S3Key:     req.S3Key,
		Label:     req.Label,
	}
	if err := s.database.SetReferenceLap(r.Context(), ref); err != nil {
		slog.Error("set reference lap", "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, ref)
}

func (s *server) handleDeleteReferenceLap(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		http.Error(w, "no database", http.StatusServiceUnavailable)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.database.DeleteReferenceLap(r.Context(), id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleReferenceLapTelemetry(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		http.Error(w, "no database", http.StatusServiceUnavailable)
		return
	}
	trackID := r.PathValue("trackId")
	carCodeStr := r.PathValue("carCode")
	carCode, err := strconv.Atoi(carCodeStr)
	if err != nil {
		http.Error(w, "invalid car_code", http.StatusBadRequest)
		return
	}
	label := r.URL.Query().Get("label")
	if label == "" {
		label = "best"
	}
	ref, err := s.database.GetReferenceLap(r.Context(), trackID, int32(carCode), label)
	if err != nil {
		http.Error(w, "reference lap not found", http.StatusNotFound)
		return
	}
	data, err := s.s3.GetObject(r.Context(), ref.S3Key)
	if err != nil {
		http.Error(w, "s3 error", http.StatusInternalServerError)
		return
	}
	rows, err := storage.ReadParquet(data)
	if err != nil {
		http.Error(w, "parquet error", http.StatusInternalServerError)
		return
	}
	downsample := 1
	if ds := r.URL.Query().Get("downsample"); ds != "" {
		if v, err := strconv.Atoi(ds); err == nil && v > 0 {
			downsample = v
		}
	}
	frames := rowsToFrames(rows, downsample)
	writeJSON(w, map[string]interface{}{
		"reference": ref,
		"frames":    frames,
		"total":     len(rows),
		"returned":  len(frames),
	})
}

func (s *server) handleCarComparison(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeJSON(w, map[string]interface{}{"tracks": []string{}, "comparisons": []interface{}{}})
		return
	}
	trackID := r.URL.Query().Get("track_id")
	if trackID == "" {
		tracks, err := s.database.GetDistinctTracks(r.Context())
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if tracks == nil {
			tracks = []string{}
		}
		writeJSON(w, map[string]interface{}{"tracks": tracks})
		return
	}
	comps, err := s.database.GetCarComparisons(r.Context(), trackID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if comps == nil {
		comps = []db.CarComparison{}
	}
	writeJSON(w, map[string]interface{}{"comparisons": comps})
}
