package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/solanyn/mono/lake/internal/config"
	icebergw "github.com/solanyn/mono/lake/internal/iceberg"
	"github.com/solanyn/mono/lake/internal/ingest"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/logging"
	"github.com/solanyn/mono/lake/internal/scheduler"
	"github.com/solanyn/mono/lake/internal/storage"
)

func main() {
	logging.Setup(os.Getenv("LOG_LEVEL"))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", "err", err)
		os.Exit(1)
	}
	s3 := storage.NewClient(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	var producer *kafka.Producer
	if cfg.KafkaBrokers != "" {
		var err error
		producer, err = kafka.NewProducer(strings.Split(cfg.KafkaBrokers, ","))
		if err != nil {
			slog.Error("kafka producer", "err", err)
			os.Exit(1)
		}
		defer producer.Close()
	}

	var iceWriter *icebergw.Writer
	if cfg.IcebergCatalogURI != "" {
		iceWriter = icebergw.NewWriter(icebergw.Config{
			CatalogURI:  cfg.IcebergCatalogURI,
			S3Endpoint:  cfg.S3Endpoint,
			S3AccessKey: cfg.S3AccessKey,
			S3SecretKey: cfg.S3SecretKey,
			S3Region:    cfg.S3Region,
		})
		slog.Info("iceberg writer enabled", "uri", cfg.IcebergCatalogURI)
	}

	sched := scheduler.New(rootCtx, cfg, s3, iceWriter, producer)
	sched.Start()
	defer sched.Stop()

	rssCollector := ingest.NewRSSCollector(s3, producer, cfg.BronzeBucket)
	rssCollector.Start()
	defer rssCollector.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		last := sched.LastIngest()
		if !last.IsZero() && time.Since(last) > 20*time.Minute {
			http.Error(w, "stale", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s3.Healthy(r.Context(), cfg.BronzeBucket) {
			http.Error(w, "s3 unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         ":" + cfg.HealthPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("lake-ingest listening", "port", cfg.HealthPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("lake-ingest: shutting down")
	rootCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
