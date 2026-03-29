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
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/promote"
	"github.com/solanyn/mono/lake/internal/storage"
)

func main() {
	cfg := storage.S3Config{
		Endpoint:  envOr("S3_ENDPOINT", "http://localhost:3900"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Region:    envOr("S3_REGION", "us-east-1"),
		Bucket:    envOr("S3_BUCKET", "datalake"),
	}
	s3 := storage.NewClient(cfg)

	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ",")

	consumer, err := kafka.NewConsumer(brokers, "lake-promote", "lake.bronze.written")
	if err != nil {
		log.Fatalf("kafka consumer: %v", err)
	}
	defer consumer.Close()

	var producer *kafka.Producer
	producer, err = kafka.NewProducer(brokers)
	if err != nil {
		log.Fatalf("kafka producer: %v", err)
	}
	defer producer.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	port := envOr("PORT", "8082")
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		log.Printf("lake-promote listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Println("lake-promote: consuming lake.bronze.written")
		if err := consumer.ConsumeBronzeWritten(ctx, func(ctx context.Context, event kafka.BronzeWritten) error {
			log.Printf("promote: received bronze event source=%s key=%s", event.Source, event.Key)
			result, err := promote.PromoteSource(ctx, s3, event.Source, event.Key)
			if err != nil {
				return err
			}
			return producer.PublishSilverWritten(ctx, kafka.SilverWritten{
				Source:    result.Source,
				Bucket:    "silver",
				Key:       result.Key,
				Timestamp: time.Now().UTC(),
				RowCount:  result.RowCount,
			})
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
