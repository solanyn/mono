package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/solanyn/mono/line/gen/linepb"
	"github.com/solanyn/mono/line/internal/coach"
	"github.com/solanyn/mono/line/internal/config"
	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/storage"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type server struct {
	s3             *storage.S3Client
	silver         *storage.S3Client
	gold           *storage.S3Client
	live           *liveHub
	coach          *coach.Pipeline
	database       *db.DB
	llm            *coach.LLMClient
	cache          *responseCache
	cars           []carEntry
	pushSubs       []webpush.Subscription
	pushMu         sync.RWMutex
	vapidPublicKey string
	vapidPrivate   string
}

func main() {
	brokers := config.EnvList("KAFKA_BROKERS", "localhost:9092", ",")
	topic := config.Env("KAFKA_TOPIC", "line.telemetry.raw")
	group := config.Env("KAFKA_GROUP", "line.api")
	s3Endpoint := config.Env("S3_ENDPOINT", "http://localhost:3900")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Region := config.Env("S3_REGION", "us-east-1")
	s3Bucket := config.Env("S3_BUCKET", "line-bronze")
	silverBucket := config.Env("S3_SILVER_BUCKET", "line-silver")
	goldBucket := config.Env("S3_GOLD_BUCKET", "line-gold")
	addr := config.Env("ADDR", ":8080")
	ttsEndpoint := config.Env("TTS_ENDPOINT", "http://mac.internal:8000")
	llmEndpoint := config.Env("LLM_ENDPOINT", "http://mac.internal:8080")
	llmModel := config.Env("LLM_MODEL", "mlx-community/gemma-4-e4b-it-4bit")
	coachVoice := config.Env("COACH_VOICE", "af_heart")
	useLLM := config.EnvBool("COACH_USE_LLM", true)
	databaseURL := config.Env("DATABASE_URL", "")
	vapidPublicKey := config.Env("VAPID_PUBLIC_KEY", "")
	vapidPrivateKey := config.Env("VAPID_PRIVATE_KEY", "")
	corsOrigin := config.Env("CORS_ORIGIN", "*")

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

	srv := &server{
		s3:             s3Client,
		silver:         silverClient,
		gold:           goldClient,
		live:           hub,
		coach:          coachPipeline,
		database:       database,
		llm:            coach.NewLLMClient(llmEndpoint, llmModel),
		cache:          newResponseCache(256, 5*time.Minute),
		vapidPublicKey: vapidPublicKey,
		vapidPrivate:   vapidPrivateKey,
	}
	srv.loadCars(ctx)
	srv.loadPushSubscriptions(ctx)

	consumer, err := kafka.NewConsumer(brokers, group, topic)
	if err != nil {
		slog.Error("failed to create kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	sessionTopic := config.Env("KAFKA_SESSION_TOPIC", "line.session.complete")
	journalGroup := config.Env("KAFKA_JOURNAL_GROUP", "line.api.journal")
	journalConsumer, err := kafka.NewConsumer(brokers, journalGroup, sessionTopic)
	if err != nil {
		slog.Warn("journal consumer not started", "err", err)
	} else {
		defer journalConsumer.Close()
		go srv.runJournalWorker(ctx, journalConsumer)
	}

	go coachPipeline.Run(ctx)
	go srv.runLiveConsumer(ctx, consumer, topic)

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
	mux.HandleFunc("GET /api/v1/sessions/{id}/journal", srv.handleGetJournal)
	mux.HandleFunc("POST /api/v1/sessions/{id}/journal", srv.handleGenerateJournal)
	mux.HandleFunc("GET /api/v1/reference-laps", srv.handleListReferenceLaps)
	mux.HandleFunc("POST /api/v1/reference-laps", srv.handleSetReferenceLap)
	mux.HandleFunc("DELETE /api/v1/reference-laps/{id}", srv.handleDeleteReferenceLap)
	mux.HandleFunc("GET /api/v1/reference-laps/{trackId}/{carCode}/telemetry", srv.handleReferenceLapTelemetry)
	mux.HandleFunc("GET /api/v1/compare", srv.handleCarComparison)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/braking", srv.handleLapBraking)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/stability", srv.handleLapStability)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps/{lap}/aligned", srv.handleLapAligned)
	mux.HandleFunc("GET /api/v1/sessions/{id}/racing-line", srv.handleRacingLine)
	mux.HandleFunc("GET /api/v1/sessions/{id}/fatigue", srv.handleFatigue)
	mux.HandleFunc("GET /api/v1/sessions/{id}/time-deltas", srv.handleTimeDeltas)
	mux.HandleFunc("GET /api/v1/push/vapid", srv.handleVAPIDKey)
	mux.HandleFunc("POST /api/v1/push/subscribe", srv.handlePushSubscribe)
	mux.HandleFunc("DELETE /api/v1/push/subscribe", srv.handlePushUnsubscribe)

	webDir := config.Env("WEB_DIR", "")
	if webDir != "" {
		spa := spaHandler{root: http.Dir(webDir)}
		mux.Handle("/", spa)
	}

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(corsOrigin)(mux),
	}

	go func() {
		<-ctx.Done()
		hub.drain()
		coachPipeline.DrainClients()
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

func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

func rowToFrame(row storage.TelemetryRow) linepb.TelemetryFrame {
	return linepb.TelemetryFrame{
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
	}
}

func rowsToFrames(rows []storage.TelemetryRow, downsample int) []linepb.TelemetryFrame {
	frames := make([]linepb.TelemetryFrame, 0, len(rows)/downsample+1)
	for i, row := range rows {
		if i%downsample != 0 {
			continue
		}
		frames = append(frames, rowToFrame(row))
	}
	return frames
}

func formatLapTimeMs(ms int) string {
	if ms <= 0 {
		return "--:--.---"
	}
	minutes := ms / 60000
	seconds := (ms % 60000) / 1000
	millis := ms % 1000
	return fmt.Sprintf("%d:%02d.%03d", minutes, seconds, millis)
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
