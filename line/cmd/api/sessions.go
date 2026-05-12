package main

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/storage"
)

func (s *server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeJSON(w, map[string]interface{}{"sessions": []interface{}{}, "next_cursor": ""})
		return
	}
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	sessions, err := s.database.ListSessions(r.Context(), limit, offset)
	if err != nil {
		slog.Error("list sessions", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []db.Session{}
	}
	writeJSON(w, map[string]interface{}{"sessions": sessions})
}

func (s *server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.database == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	session, err := s.database.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, session)
}

func (s *server) handleListLaps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.database == nil {
		writeJSON(w, map[string]interface{}{"laps": []interface{}{}})
		return
	}
	laps, err := s.database.ListLaps(r.Context(), id)
	if err != nil {
		slog.Error("list laps", "session", id, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if laps == nil {
		laps = []db.Lap{}
	}
	writeJSON(w, map[string]interface{}{"laps": laps})
}

func (s *server) handleGetTelemetry(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	downsample := 1
	if ds := r.URL.Query().Get("downsample"); ds != "" {
		if v, err := strconv.Atoi(ds); err == nil && v > 0 {
			downsample = v
		}
	}

	data, err := s.s3.GetLap(r.Context(), sessionID, lapNum)
	if err != nil {
		slog.Error("get lap from s3", "session", sessionID, "lap", lapNum, "err", err)
		http.Error(w, "lap not found", http.StatusNotFound)
		return
	}

	rows, err := storage.ReadParquet(data)
	if err != nil {
		slog.Error("read parquet", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	frames := rowsToFrames(rows, downsample)

	writeJSON(w, map[string]interface{}{
		"session_id": sessionID,
		"lap_number": lapNum,
		"frames":     frames,
		"total":      len(rows),
		"returned":   len(frames),
	})
}

func (s *server) handleProgression(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeJSON(w, map[string]interface{}{"points": []interface{}{}})
		return
	}
	trackID := r.URL.Query().Get("track_id")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	points, err := s.database.GetProgression(r.Context(), trackID, limit)
	if err != nil {
		slog.Error("get progression", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if points == nil {
		points = []db.ProgressionPoint{}
	}

	writeJSON(w, map[string]interface{}{"points": points})
}
