package main

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/solanyn/mono/line/gen/linepb"
)

func TestGetSession(t *testing.T) {
	handler := &grpcWebHandler{server: &lineServer{}}
	body := strings.NewReader(`{"session_id":"sess-001"}`)
	req := httptest.NewRequest("POST", "/line.v1.LineService/GetSession", body)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var sess linepb.Session
	json.NewDecoder(w.Body).Decode(&sess)

	if sess.Id != "sess-001" {
		t.Fatalf("expected sess-001, got %s", sess.Id)
	}
	if sess.CarCode != 3317 {
		t.Fatalf("expected car code 3317, got %d", sess.CarCode)
	}
	if sess.TrackId != "tsukuba" {
		t.Fatalf("expected tsukuba, got %s", sess.TrackId)
	}
}

func TestListSessions(t *testing.T) {
	handler := &grpcWebHandler{server: &lineServer{}}
	body := strings.NewReader(`{"limit":10}`)
	req := httptest.NewRequest("POST", "/line.v1.LineService/ListSessions", body)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Sessions   []linepb.Session `json:"sessions"`
		NextCursor string           `json:"next_cursor"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.Sessions))
	}
}

func TestStreamTelemetry(t *testing.T) {
	handler := &grpcWebHandler{server: &lineServer{}}
	body := strings.NewReader(`{"session_id":"sess-001","lap_number":1}`)
	req := httptest.NewRequest("POST", "/line.v1.LineService/StreamTelemetry", body)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 600 {
		t.Fatalf("expected 600 frames, got %d", len(lines))
	}

	var frame linepb.TelemetryFrame
	json.Unmarshal([]byte(lines[0]), &frame)

	if frame.Speed == 0 {
		t.Fatal("expected non-zero speed")
	}
	if frame.Gear == 0 {
		t.Fatal("expected non-zero gear")
	}
}

func TestCORS(t *testing.T) {
	handler := &grpcWebHandler{server: &lineServer{}}
	req := httptest.NewRequest("OPTIONS", "/line.v1.LineService/GetSession", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

func TestNotFound(t *testing.T) {
	handler := &grpcWebHandler{server: &lineServer{}}
	req := httptest.NewRequest("POST", "/line.v1.LineService/Unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "not found") {
		t.Fatalf("expected 'not found', got %s", string(body))
	}
}
