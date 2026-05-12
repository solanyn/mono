package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

func (s *server) handleLapMetrics(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	key := "laps/" + sessionID + "/" + strconv.Itoa(lapNum) + "/metrics.json"
	data, err := s.silver.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "metrics not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *server) handleSessionSummary(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	data, err := s.gold.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *server) handleLapBraking(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	key := "laps/" + sessionID + "/" + fmt.Sprintf("%03d", lapNum) + "/metrics.json"
	raw, err := s.silver.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "metrics not found", http.StatusNotFound)
		return
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(raw, &metrics); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	braking, ok := metrics["braking"]
	if !ok {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, braking)
}

func (s *server) handleLapStability(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	key := "laps/" + sessionID + "/" + fmt.Sprintf("%03d", lapNum) + "/metrics.json"
	raw, err := s.silver.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "metrics not found", http.StatusNotFound)
		return
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(raw, &metrics); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	stability, ok := metrics["stability"]
	if !ok {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, stability)
}

func (s *server) handleLapAligned(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	lapStr := r.PathValue("lap")
	lapNum, err := strconv.Atoi(lapStr)
	if err != nil {
		http.Error(w, "invalid lap number", http.StatusBadRequest)
		return
	}

	key := "laps/" + sessionID + "/" + fmt.Sprintf("%03d", lapNum) + "/metrics.json"
	raw, err := s.silver.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "metrics not found", http.StatusNotFound)
		return
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(raw, &metrics); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	aligned, ok := metrics["aligned"]
	if !ok {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, aligned)
}

func (s *server) handleRacingLine(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.gold.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	var summary map[string]interface{}
	if err := json.Unmarshal(raw, &summary); err != nil {
		slog.Error("parse summary", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	racingLine, ok := summary["racing_line"]
	if !ok {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, racingLine)
}

func (s *server) handleFatigue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.gold.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	var summary map[string]interface{}
	if err := json.Unmarshal(raw, &summary); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	fatigue, ok := summary["fatigue"]
	if !ok {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, fatigue)
}

func (s *server) handleTimeDeltas(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.gold.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	var summary map[string]interface{}
	if err := json.Unmarshal(raw, &summary); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	deltas, ok := summary["time_deltas"]
	if !ok {
		writeJSON(w, map[string]interface{}{"deltas": []interface{}{}})
		return
	}
	writeJSON(w, map[string]interface{}{"deltas": deltas})
}
