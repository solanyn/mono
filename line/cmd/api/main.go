package main

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/gen/linepb"
	"github.com/solanyn/mono/line/internal/coach"
	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/storage"
	"github.com/solanyn/mono/line/internal/telemetry"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type server struct {
	s3       *storage.S3Client
	silver   *storage.S3Client
	gold     *storage.S3Client
	live     *liveHub
	coach    *coach.Pipeline
	database *db.DB
}

type liveHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
	status  linepb.LiveStatus
}

func newLiveHub() *liveHub {
	return &liveHub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

func (h *liveHub) broadcast(frame *linepb.TelemetryFrame) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, _ := json.Marshal(frame)
	for conn := range h.clients {
		conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			go h.remove(conn)
		}
	}
}

func (h *liveHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *liveHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *liveHub) setStatus(s linepb.LiveStatus) {
	h.mu.Lock()
	h.status = s
	h.mu.Unlock()
}

func (h *liveHub) getStatus() linepb.LiveStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func main() {
	brokers := strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ",")
	topic := envOrDefault("KAFKA_TOPIC", "line.telemetry.raw")
	group := envOrDefault("KAFKA_GROUP", "line.api")
	s3Endpoint := envOrDefault("S3_ENDPOINT", "http://localhost:3900")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Region := envOrDefault("S3_REGION", "us-east-1")
	s3Bucket := envOrDefault("S3_BUCKET", "line-bronze")
	silverBucket := envOrDefault("S3_SILVER_BUCKET", "line-silver")
	goldBucket := envOrDefault("S3_GOLD_BUCKET", "line-gold")
	addr := envOrDefault("ADDR", ":8080")
	ttsEndpoint := envOrDefault("TTS_ENDPOINT", "http://mac.internal:8000")
	llmEndpoint := envOrDefault("LLM_ENDPOINT", "http://mac.internal:8080")
	llmModel := envOrDefault("LLM_MODEL", "mlx-community/gemma-4-e4b-it-4bit")
	coachVoice := envOrDefault("COACH_VOICE", "af_heart")
	useLLM := envOrDefault("COACH_USE_LLM", "true") == "true"
	databaseURL := envOrDefault("DATABASE_URL", "")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	s3Client := storage.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3Region, s3Bucket)
	silverClient := storage.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3Region, silverBucket)
	goldClient := storage.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3Region, goldBucket)
	hub := newLiveHub()

	var database *db.DB
	if databaseURL != "" {
		var err error
		database, err = db.New(ctx, databaseURL)
		if err != nil {
			slog.Error("failed to connect to database", "err", err)
			os.Exit(1)
		}
		defer database.Close()
		slog.Info("database connected")
	} else {
		slog.Warn("DATABASE_URL not set, session queries will return empty")
	}

	coachPipeline := coach.NewPipeline(coach.Config{
		TTSEndpoint: ttsEndpoint,
		LLMEndpoint: llmEndpoint,
		LLMModel:    llmModel,
		Voice:       coachVoice,
		Speed:       1.0,
		UseLLM:      useLLM,
	})

	srv := &server{s3: s3Client, silver: silverClient, gold: goldClient, live: hub, coach: coachPipeline, database: database}

	consumer, err := kafka.NewConsumer(brokers, group, topic)
	if err != nil {
		slog.Error("failed to create kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	go coachPipeline.Run(ctx)

	go func() {
		slog.Info("live consumer started", "topic", topic)
		var currentCar int32
		var currentLap int32
		consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
			if len(record.Value) < telemetry.EncodedFrameSize {
				return nil
			}
			frame := telemetry.DecodeFrame(record.Value)
			if !frame.IsOnTrack() || frame.IsPaused() {
				return nil
			}

			if frame.CarID != currentCar || int32(frame.CurrentLap) != currentLap {
				currentCar = frame.CarID
				currentLap = int32(frame.CurrentLap)
				hub.setStatus(linepb.LiveStatus{
					Active:     true,
					CarCode:    currentCar,
					CurrentLap: currentLap,
				})
			}

			tf := &linepb.TelemetryFrame{
				PacketID:    frame.PacketID,
				X:           frame.PosX,
				Y:           frame.PosY,
				Z:           frame.PosZ,
				Speed:       frame.Speed,
				Throttle:    float32(frame.Throttle) / 255.0,
				Brake:       float32(frame.Brake) / 255.0,
				Steering:    float32(frame.Steering) / 127.0,
				Rpm:         frame.RPM,
				Gear:        int32(frame.Gear),
				TireTempFL:  frame.TireTempFL,
				TireTempFR:  frame.TireTempFR,
				TireTempRL:  frame.TireTempRL,
				TireTempRR:  frame.TireTempRR,
				FuelLevel:   frame.FuelLevel,
				CurrentLap:  int32(frame.CurrentLap),
				CurrentTime: frame.CurrentTime,
				TimestampNs: record.Timestamp.UnixNano(),
			}
			hub.broadcast(tf)
			coachPipeline.Detector().Process(ctx, frame)
			return nil
		})
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)
	mux.HandleFunc("GET /api/v1/status", srv.handleStatus)
	mux.HandleFunc("GET /api/v1/sessions", srv.handleListSessions)
	mux.HandleFunc("GET /api/v1/sessions/{id}", srv.handleGetSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps", srv.handleListLaps)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/telemetry", srv.handleGetTelemetry)
	mux.HandleFunc("GET /api/v1/live", srv.handleLiveWS)
	mux.HandleFunc("GET /api/v1/coach", srv.handleCoachWS)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/metrics", srv.handleLapMetrics)
	mux.HandleFunc("GET /api/v1/sessions/{id}/summary", srv.handleSessionSummary)
	mux.HandleFunc("GET /api/v1/tracks", srv.handleListTracks)

	webDir := envOrDefault("WEB_DIR", "")
	if webDir != "" {
		spa := spaHandler{root: http.Dir(webDir)}
		mux.Handle("/", spa)
	}

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	slog.Info("api server started", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
	slog.Info("api server stopped")
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.live.getStatus())
}

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

	frames := make([]linepb.TelemetryFrame, 0, len(rows)/downsample+1)
	for i, row := range rows {
		if i%downsample != 0 {
			continue
		}
		frames = append(frames, linepb.TelemetryFrame{
			PacketID:    row.PacketID,
			X:           row.PosX,
			Y:           row.PosY,
			Z:           row.PosZ,
			Speed:       row.Speed,
			Throttle:    float32(row.Throttle) / 255.0,
			Brake:       float32(row.Brake) / 255.0,
			Steering:    float32(row.Steering) / 127.0,
			Rpm:         row.RPM,
			Gear:        row.Gear,
			TireTempFL:  row.TireTempFL,
			TireTempFR:  row.TireTempFR,
			TireTempRL:  row.TireTempRL,
			TireTempRR:  row.TireTempRR,
			FuelLevel:   row.FuelLevel,
			CurrentLap:  row.CurrentLap,
			CurrentTime: row.CurrentTime,
		})
	}

	writeJSON(w, map[string]interface{}{
		"session_id": sessionID,
		"lap_number": lapNum,
		"frames":     frames,
		"total":      len(rows),
		"returned":   len(frames),
	})
}

func (s *server) handleLiveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "err", err)
		return
	}
	defer conn.Close()

	s.live.add(conn)
	defer s.live.remove(conn)

	status, _ := json.Marshal(s.live.getStatus())
	conn.WriteMessage(websocket.TextMessage, status)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *server) handleCoachWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("coach websocket upgrade", "err", err)
		return
	}
	defer conn.Close()

	s.coach.AddClient(conn)
	defer s.coach.RemoveClient(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

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

func (s *server) handleListTracks(w http.ResponseWriter, r *http.Request) {
	tracksFile := envOrDefault("TRACKS_DATA", "")
	if tracksFile == "" {
		writeJSON(w, map[string]interface{}{"tracks": []interface{}{}})
		return
	}
	data, err := os.ReadFile(tracksFile)
	if err != nil {
		http.Error(w, "tracks data not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type spaHandler struct {
	root http.FileSystem
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	f, err := h.root.Open(path)
	if err != nil {
		if pathErr, ok := err.(*fs.PathError); ok && pathErr.Err == fs.ErrNotExist {
			index, _ := h.root.Open("/index.html")
			if index != nil {
				defer index.Close()
				stat, _ := index.Stat()
				http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
				return
			}
		}
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	if stat.IsDir() {
		index, err := h.root.Open(path + "/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer index.Close()
		stat, _ = index.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
		return
	}
	http.ServeContent(w, r, path, stat.ModTime(), f.(io.ReadSeeker))
}
