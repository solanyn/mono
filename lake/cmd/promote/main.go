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
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/logging"
	"github.com/solanyn/mono/lake/internal/promote"
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

	if cfg.IcebergCatalogURI == "" {
		slog.Error("config: ICEBERG_CATALOG_URI required for promote")
		os.Exit(1)
	}
	iceWriter := icebergw.NewWriter(icebergw.Config{
		CatalogURI:  cfg.IcebergCatalogURI,
		S3Endpoint:  cfg.S3Endpoint,
		S3AccessKey: cfg.S3AccessKey,
		S3SecretKey: cfg.S3SecretKey,
		S3Region:    cfg.S3Region,
	})
	slog.Info("iceberg writer enabled", "uri", cfg.IcebergCatalogURI)

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	consumer, err := kafka.NewConsumer(brokers, "lake-promote", cfg.KafkaTopicBronze)
	if err != nil {
		slog.Error("kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		slog.Error("kafka producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
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

	srv := &http.Server{Addr: ":" + cfg.HealthPort, Handler: mux}
	go func() {
		slog.Info("lake-promote listening", "port", cfg.HealthPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		slog.Info("lake-promote: consuming", "topic", cfg.KafkaTopicBronze)
		if err := consumer.ConsumeBronzeWritten(ctx, func(ctx context.Context, event kafka.BronzeWritten) error {
			slog.Info("promote: received bronze event", "source", event.Source, "key", event.Key)
			result, err := promote.PromoteSource(ctx, s3, iceWriter, cfg.BronzeBucket, event.Source, event.Key)
			if err != nil {
				return err
			}
			if result.RowCount == 0 {
				return nil
			}
			return producer.PublishSilverWritten(ctx, kafka.SilverWritten{
				Source:    result.Source,
				Table:     result.Table,
				BronzeKey: event.Key,
				Timestamp: time.Now().UTC(),
				RowCount:  result.RowCount,
			})
		}); err != nil && ctx.Err() == nil {
			slog.Error("consumer", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
