package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/solanyn/mono/lake/internal/aggregate"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/storage"
)

func main() {
	cfg := storage.S3Config{
		Endpoint:  envOr("S3_ENDPOINT", "http://localhost:3900"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Region:    envOr("S3_REGION", "us-east-1"),
	}
	s3 := storage.NewClient(cfg)

	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ",")

	consumer, err := kafka.NewConsumer(brokers, "lake-aggregate", "lake.silver.written")
	if err != nil {
		log.Fatalf("kafka consumer: %v", err)
	}
	defer consumer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	port := envOr("PORT", "8083")
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		log.Printf("lake-aggregate listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Println("lake-aggregate: consuming lake.silver.written")
		if err := consumer.ConsumeSilverWritten(ctx, func(ctx context.Context, event kafka.SilverWritten) error {
			log.Printf("aggregate: received silver event source=%s key=%s", event.Source, event.Key)
			return aggregate.SilverToGold(ctx, s3, event.Source, event.Key)
		}); err != nil && ctx.Err() == nil {
			log.Fatalf("consumer: %v", err)
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
