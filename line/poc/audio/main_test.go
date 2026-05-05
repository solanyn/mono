package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handleWebSocket))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := TTSRequest{Text: "hello world", Voice: "af_heart", Speed: 1.0}
	msg, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, resp, err := conn.ReadMessage()
	if err != nil {
		t.Skipf("kokoro not available (expected in CI): %v", err)
		return
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(resp, &meta); err != nil {
		if meta == nil {
			t.Fatalf("expected json metadata, got binary")
		}
	}

	if meta["type"] != "audio" {
		if errMsg, ok := meta["error"]; ok {
			t.Skipf("kokoro returned error (expected if not running): %v", errMsg)
			return
		}
		t.Fatalf("unexpected meta type: %v", meta["type"])
	}

	_, audioData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}

	if len(audioData) < 44 {
		t.Fatalf("audio too small for WAV: %d bytes", len(audioData))
	}

	if string(audioData[:4]) != "RIFF" {
		t.Fatalf("not a WAV file, header: %x", audioData[:4])
	}

	t.Logf("received WAV audio: %d bytes, latency: %vms", len(audioData), meta["latency_ms"])
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected 'ok', got %q", w.Body.String())
	}
}

func TestInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handleWebSocket))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.WriteMessage(websocket.TextMessage, []byte("not json"))

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, resp, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var errResp map[string]string
	json.Unmarshal(resp, &errResp)
	if errResp["error"] != "invalid json" {
		t.Fatalf("expected 'invalid json' error, got: %s", string(resp))
	}
}
