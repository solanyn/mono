package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/solanyn/mono/line/gen/linepb"
)

func TestHealthEndpoint(t *testing.T) {
	srv := &server{live: newLiveHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected ok, got %s", w.Body.String())
	}
}

func TestStatusEndpoint(t *testing.T) {
	hub := newLiveHub()
	hub.setStatus(linepb.LiveStatus{
		Active:     true,
		CarCode:    3317,
		CurrentLap: 5,
	})
	srv := &server{live: hub}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", srv.handleStatus)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status linepb.LiveStatus
	json.NewDecoder(w.Body).Decode(&status)
	if !status.Active {
		t.Fatal("expected active")
	}
	if status.CarCode != 3317 {
		t.Fatalf("expected car 3317, got %d", status.CarCode)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	srv := &server{live: newLiveHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/sessions", srv.handleListSessions)

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 0 {
		t.Fatalf("expected empty sessions, got %d", len(sessions))
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := &server{live: newLiveHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)
	handler := corsMiddleware(mux)

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}

func TestLiveWebSocket(t *testing.T) {
	hub := newLiveHub()
	hub.setStatus(linepb.LiveStatus{Active: false})
	srv := &server{live: hub}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/live", srv.handleLiveWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + ts.URL[4:] + "/api/v1/live"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var status linepb.LiveStatus
	json.Unmarshal(msg, &status)
	if status.Active {
		t.Fatal("expected inactive status on connect")
	}

	hub.broadcast(&linepb.TelemetryFrame{Speed: 150.5, Gear: 4})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}

	var frame linepb.TelemetryFrame
	json.Unmarshal(msg, &frame)
	if frame.Speed != 150.5 {
		t.Fatalf("expected speed 150.5, got %f", frame.Speed)
	}
	if frame.Gear != 4 {
		t.Fatalf("expected gear 4, got %d", frame.Gear)
	}
}

func TestLiveHubConcurrentBroadcast(t *testing.T) {
	hub := newLiveHub()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.add(conn)
		defer hub.remove(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer ts.Close()

	const numClients = 10
	conns := make([]*websocket.Conn, numClients)
	for i := range conns {
		wsURL := "ws" + ts.URL[4:]
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		conns[i] = c
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.broadcast(&linepb.TelemetryFrame{PacketID: int32(n), Speed: float32(n)})
		}(i)
	}

	conns[0].Close()
	conns[1].Close()

	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	remaining := len(hub.clients)
	hub.mu.RUnlock()

	if remaining > numClients {
		t.Errorf("expected <= %d clients, got %d", numClients, remaining)
	}

	for _, c := range conns[2:] {
		c.Close()
	}
}
