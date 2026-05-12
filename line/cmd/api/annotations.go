package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/solanyn/mono/line/internal/db"
)

func (s *server) handleListAnnotations(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeJSON(w, map[string]interface{}{"annotations": []interface{}{}})
		return
	}
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}
	annotations, err := s.database.ListAnnotations(r.Context(), sessionID, lapNum)
	if err != nil {
		slog.Error("list annotations", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if annotations == nil {
		annotations = []db.Annotation{}
	}
	writeJSON(w, map[string]interface{}{"annotations": annotations})
}

func (s *server) handleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	var body struct {
		FrameIdx int    `json:"frame_idx"`
		Text     string `json:"text"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	a := &db.Annotation{
		SessionID: sessionID,
		LapNumber: lapNum,
		FrameIdx:  body.FrameIdx,
		Text:      body.Text,
	}
	if err := s.database.CreateAnnotation(r.Context(), a); err != nil {
		slog.Error("create annotation", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, a)
}

func (s *server) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.database.DeleteAnnotation(r.Context(), id); err != nil {
		slog.Error("delete annotation", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
