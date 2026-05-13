package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/solanyn/mono/line/internal/storage"
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
	data, err := s.getCachedS3(r, s.silver, "silver:"+key, key)
	if err != nil {
		http.Error(w, "metrics not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Write(data)
}

func (s *server) handleSessionSummary(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	data, err := s.getCachedS3(r, s.gold, "gold:"+key, key)
	if err != nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
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
	raw, err := s.getCachedS3(r, s.silver, "silver:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
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
	raw, err := s.getCachedS3(r, s.silver, "silver:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
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
	raw, err := s.getCachedS3(r, s.silver, "silver:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	writeJSON(w, aligned)
}

func (s *server) handleRacingLine(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.getCachedS3(r, s.gold, "gold:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, racingLine)
}

func (s *server) handleFatigue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.getCachedS3(r, s.gold, "gold:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, fatigue)
}

func (s *server) handleTimeDeltas(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	key := "sessions/" + sessionID + "/summary.json"
	raw, err := s.getCachedS3(r, s.gold, "gold:"+key, key)
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
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, map[string]interface{}{"deltas": deltas})
}

func (s *server) getCachedS3(r *http.Request, client *storage.S3Client, cacheKey, s3Key string) ([]byte, error) {
	if data, ok := s.cache.Get(cacheKey); ok {
		return data, nil
	}
	data, err := client.GetObject(r.Context(), s3Key)
	if err != nil {
		return nil, err
	}
	s.cache.Set(cacheKey, data)
	return data, nil
}
