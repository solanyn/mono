package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/parquet-go/parquet-go"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/data"
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
	cars     []carEntry
}

type carEntry struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Maker   string `json:"maker"`
	Country string `json:"country"`
	Group   string `json:"group,omitempty"`
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
	srv.loadCars(ctx)

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
	mux.HandleFunc("GET /api/v1/cars", srv.handleListCars)
	mux.HandleFunc("GET /api/v1/cars/{id}", srv.handleGetCar)
	mux.HandleFunc("GET /api/v1/progression", srv.handleProgression)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/annotations", srv.handleListAnnotations)
	mux.HandleFunc("POST /api/v1/sessions/{id}/laps/{lap}/annotations", srv.handleCreateAnnotation)
	mux.HandleFunc("DELETE /api/v1/annotations/{id}", srv.handleDeleteAnnotation)
	mux.HandleFunc("POST /api/v1/sessions/{id}/briefing", srv.handleGenerateBriefing)

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
	w.Header().Set("Content-Type", "application/json")
	w.Write(data.TracksJSON)
}

func (s *server) handleListCars(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cars)
}

func (s *server) handleGetCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid car id", http.StatusBadRequest)
		return
	}
	for _, c := range s.cars {
		if c.ID == id {
			writeJSON(w, c)
			return
		}
	}
	http.Error(w, "car not found", http.StatusNotFound)
}

func (s *server) loadCars(ctx context.Context) {
	if s.s3 == nil {
		slog.Warn("no S3 client configured, cars unavailable")
		return
	}
	carsData, err := s.s3.GetLatestByPrefix(ctx, "reference/cars/")
	if err != nil {
		slog.Warn("failed to load cars from S3", "err", err)
		return
	}
	rows, err := readCarsParquet(carsData)
	if err != nil {
		slog.Warn("failed to parse cars parquet", "err", err)
		return
	}
	s.cars = rows
	slog.Info("loaded cars from S3", "count", len(rows))
}

func readCarsParquet(buf []byte) ([]carEntry, error) {
	type parquetCar struct {
		ID      int32  `parquet:"id"`
		Name    string `parquet:"name"`
		Maker   string `parquet:"maker"`
		Country string `parquet:"country"`
		Group   string `parquet:"group"`
	}
	reader := parquet.NewGenericReader[parquetCar](bytes.NewReader(buf))
	defer reader.Close()

	rows := make([]parquetCar, reader.NumRows())
	n, err := reader.Read(rows)
	if err != nil && err != io.EOF {
		return nil, err
	}
	cars := make([]carEntry, n)
	for i := 0; i < n; i++ {
		cars[i] = carEntry{
			ID:      int(rows[i].ID),
			Name:    rows[i].Name,
			Maker:   rows[i].Maker,
			Country: rows[i].Country,
			Group:   rows[i].Group,
		}
	}
	return cars, nil
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

	for i := range points {
		times, err := s.database.GetLapTimesForSession(r.Context(), points[i].SessionID)
		if err == nil && len(times) > 1 {
			mean := float64(0)
			for _, t := range times {
				mean += float64(t)
			}
			mean /= float64(len(times))
			variance := float64(0)
			for _, t := range times {
				d := float64(t) - mean
				variance += d * d
			}
			variance /= float64(len(times))
			cv := 0.0
			if mean > 0 {
				cv = (variance / (mean * mean))
			}
			score := 1.0 - cv*100
			if score < 0 {
				score = 0
			}
			points[i].ConsistencyScore = &score
		}
	}

	writeJSON(w, map[string]interface{}{"points": points})
}

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

func (s *server) handleGenerateBriefing(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	summaryKey := "sessions/" + sessionID + "/summary.json"
	summaryData, err := s.gold.GetObject(r.Context(), summaryKey)
	if err != nil {
		http.Error(w, "session summary not found — complete a session first", http.StatusNotFound)
		return
	}

	var summary struct {
		CarCode     int32  `json:"car_code"`
		TrackName   string `json:"track_name"`
		LapCount    int    `json:"lap_count"`
		Consistency struct {
			ConsistencyScore  float64 `json:"consistency_score"`
			LapTimeCV         float64 `json:"lap_time_cv"`
			BestWorstDeltaMs  int     `json:"best_worst_delta_ms"`
		} `json:"consistency"`
		TyreDegradation struct {
			DegradationRate      float64 `json:"degradation_rate"`
			EstimatedLapsRemain  int     `json:"estimated_laps_remaining"`
			CompoundGuess        string  `json:"compound_guess"`
			FrontRearBalance     float64 `json:"front_rear_balance"`
		} `json:"tyre_degradation"`
		FuelStrategy struct {
			ConsumptionPerLap float64 `json:"consumption_per_lap"`
			LapsRemaining     int     `json:"laps_remaining"`
			OptimalPitLap     int     `json:"optimal_pit_lap"`
		} `json:"fuel_strategy"`
		Journal struct {
			BestLapMs       int      `json:"best_lap_ms"`
			WorstLapMs      int      `json:"worst_lap_ms"`
			Highlights      []string `json:"highlights"`
			AreasToImprove  []string `json:"areas_to_improve"`
			CornerNotes     []string `json:"corner_notes"`
		} `json:"journal"`
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		slog.Error("parse summary for briefing", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	llmEndpoint := envOrDefault("LLM_ENDPOINT", "http://mac.internal:8080")
	llmModel := envOrDefault("LLM_MODEL", "mlx-community/gemma-4-e4b-it-4bit")
	llm := coach.NewLLMClient(llmEndpoint, llmModel)

	prompt := buildBriefingPrompt(summary.TrackName, summary.CarCode, summary.LapCount,
		summary.Consistency.ConsistencyScore, summary.Consistency.LapTimeCV, summary.Consistency.BestWorstDeltaMs,
		summary.Journal.BestLapMs, summary.Journal.WorstLapMs,
		summary.TyreDegradation.DegradationRate, summary.TyreDegradation.CompoundGuess, summary.TyreDegradation.FrontRearBalance,
		summary.FuelStrategy.ConsumptionPerLap, summary.FuelStrategy.LapsRemaining, summary.FuelStrategy.OptimalPitLap,
		summary.Journal.Highlights, summary.Journal.AreasToImprove, summary.Journal.CornerNotes)

	briefing, err := llm.GenerateBriefing(r.Context(), prompt)
	if err != nil {
		slog.Error("llm briefing generation", "err", err)
		http.Error(w, "briefing generation failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"session_id": sessionID,
		"briefing":   briefing,
	})
}

func buildBriefingPrompt(track string, carCode int32, lapCount int,
	consistency, lapTimeCV float64, bestWorstDelta int,
	bestLapMs, worstLapMs int,
	tyreDegrad float64, compound string, frBalance float64,
	fuelPerLap float64, fuelLapsRemain, pitLap int,
	highlights, areasToImprove, cornerNotes []string) string {

	var b strings.Builder
	b.WriteString("Generate a pre-race briefing for the driver. Be concise, actionable, and specific.\n\n")
	b.WriteString("SESSION DATA:\n")
	b.WriteString("Track: " + track + "\n")
	b.WriteString("Car code: " + strconv.Itoa(int(carCode)) + "\n")
	b.WriteString("Laps completed: " + strconv.Itoa(lapCount) + "\n")
	b.WriteString("Best lap: " + formatLapTimeMs(bestLapMs) + "\n")
	b.WriteString("Worst lap: " + formatLapTimeMs(worstLapMs) + "\n")
	b.WriteString("Consistency score: " + strconv.FormatFloat(consistency*100, 'f', 1, 64) + "%\n")
	b.WriteString("Lap time CV: " + strconv.FormatFloat(lapTimeCV*100, 'f', 2, 64) + "%\n")
	b.WriteString("Best-worst delta: " + strconv.Itoa(bestWorstDelta) + "ms\n\n")

	b.WriteString("TYRE:\n")
	b.WriteString("Compound: " + compound + "\n")
	b.WriteString("Degradation rate: " + strconv.FormatFloat(tyreDegrad, 'f', 2, 64) + " C/lap\n")
	b.WriteString("Front/rear balance: " + strconv.FormatFloat(frBalance, 'f', 2, 64) + "\n\n")

	b.WriteString("FUEL:\n")
	b.WriteString("Consumption: " + strconv.FormatFloat(fuelPerLap, 'f', 2, 64) + " L/lap\n")
	b.WriteString("Range: " + strconv.Itoa(fuelLapsRemain) + " laps\n")
	if pitLap > 0 {
		b.WriteString("Optimal pit: lap " + strconv.Itoa(pitLap) + "\n")
	}
	b.WriteString("\n")

	if len(highlights) > 0 {
		b.WriteString("STRENGTHS:\n")
		for _, h := range highlights {
			b.WriteString("- " + h + "\n")
		}
		b.WriteString("\n")
	}

	if len(areasToImprove) > 0 {
		b.WriteString("AREAS TO IMPROVE:\n")
		for _, a := range areasToImprove {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}

	if len(cornerNotes) > 0 {
		b.WriteString("CORNER NOTES:\n")
		for _, n := range cornerNotes {
			b.WriteString("- " + n + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("INSTRUCTIONS:\n")
	b.WriteString("Write a focused pre-race briefing (3-5 paragraphs) covering:\n")
	b.WriteString("1. Target lap time and realistic expectations\n")
	b.WriteString("2. Key corners to focus on (reference specific weaknesses)\n")
	b.WriteString("3. Tyre management approach for this stint\n")
	b.WriteString("4. Fuel strategy if relevant\n")
	b.WriteString("5. One specific technique to practice this session\n")
	b.WriteString("Address the driver as 'you'. Be direct and motivating without being cheesy.")

	return b.String()
}

func formatLapTimeMs(ms int) string {
	if ms <= 0 {
		return "--:--.---"
	}
	minutes := ms / 60000
	seconds := (ms % 60000) / 1000
	millis := ms % 1000
	return strconv.Itoa(minutes) + ":" + fmt.Sprintf("%02d.%03d", seconds, millis)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
