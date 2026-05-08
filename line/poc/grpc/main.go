package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/solanyn/mono/line/gen/linepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type lineServer struct{}

func (s *lineServer) GetSession(ctx context.Context, id string) (*linepb.Session, error) {
	return &linepb.Session{
		Id:        id,
		StartedAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
		EndedAt:   time.Now().UnixMilli(),
		CarCode:   3317,
		TrackId:   "tsukuba",
		LapCount:  12,
		BestLapMs: 62450,
	}, nil
}

func (s *lineServer) ListSessions(limit int32, cursor string) ([]*linepb.Session, string, error) {
	sessions := []*linepb.Session{
		{Id: "sess-001", StartedAt: time.Now().Add(-2 * time.Hour).UnixMilli(), CarCode: 3317, TrackId: "tsukuba", LapCount: 12, BestLapMs: 62450},
		{Id: "sess-002", StartedAt: time.Now().Add(-24 * time.Hour).UnixMilli(), CarCode: 2891, TrackId: "suzuka", LapCount: 8, BestLapMs: 121300},
	}
	return sessions, "", nil
}

func (s *lineServer) StreamTelemetry(sessionID string, lap int32, send func(*linepb.TelemetryFrame) error) error {
	numFrames := 600
	for i := 0; i < numFrames; i++ {
		t := float64(i) / float64(numFrames) * math.Pi * 2
		r := 200 + 60*math.Sin(t*2)
		frame := &linepb.TelemetryFrame{
			X:           float32(math.Cos(t) * r),
			Y:           float32(5 * math.Sin(t*3)),
			Z:           float32(math.Sin(t) * r),
			Speed:       float32(180 + 80*math.Cos(t*3)),
			Throttle:    float32(math.Max(0, math.Cos(t*3))),
			Brake:       float32(math.Max(0, -math.Cos(t*3))),
			Steering:    float32(math.Sin(t * 2) * 0.5),
			Rpm:         float32(4000 + 3000*math.Cos(t*3)),
			Gear:        int32(3 + 2*math.Cos(t*2)),
			TimestampNs: int64(i) * 16666667,
		}
		if err := send(frame); err != nil {
			return err
		}
	}
	return nil
}

type grpcWebHandler struct {
	server *lineServer
}

func (h *grpcWebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Grpc-Web, X-User-Agent")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/line.v1.LineService/")
	w.Header().Set("Content-Type", "application/json")

	switch path {
	case "GetSession":
		var req struct {
			SessionId string `json:"session_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		sess, err := h.server.GetSession(r.Context(), req.SessionId)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(sess)

	case "ListSessions":
		var req struct {
			Limit  int32  `json:"limit"`
			Cursor string `json:"cursor"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		sessions, cursor, err := h.server.ListSessions(req.Limit, req.Cursor)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions":    sessions,
			"next_cursor": cursor,
		})

	case "StreamTelemetry":
		var req struct {
			SessionId string `json:"session_id"`
			LapNumber int32  `json:"lap_number"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", 500)
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		h.server.StreamTelemetry(req.SessionId, req.LapNumber, func(frame *linepb.TelemetryFrame) error {
			data, _ := json.Marshal(frame)
			_, err := fmt.Fprintf(w, "%s\n", data)
			flusher.Flush()
			return err
		})

	default:
		http.Error(w, "not found", 404)
	}
}

func main() {
	srv := &lineServer{}

	gs := grpc.NewServer()
	reflection.Register(gs)

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	go func() {
		log.Printf("gRPC server on :9090")
		if err := gs.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	httpHandler := &grpcWebHandler{server: srv}
	httpMux := http.NewServeMux()
	httpMux.Handle("/line.v1.LineService/", httpHandler)
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	log.Printf("grpc-web HTTP proxy on :8091")
	if err := http.ListenAndServe(":8091", httpMux); err != nil {
		log.Fatal(err)
	}
}
