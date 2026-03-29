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
	"github.com/solanyn/mono/lake/internal/config"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/promote"
	"github.com/solanyn/mono/lake/internal/storage"
)

func main() {
	cfg := config.Load()
	s3 := storage.NewClient(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region)

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	consumer, err := kafka.NewConsumer(brokers, "lake-promote", cfg.KafkaTopicBronze)
	if err != nil {
		log.Fatalf("kafka consumer: %v", err)
	}
	defer consumer.Close()

	producer, err := kafka.NewProducer(brokers)
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

	srv := &http.Server{Addr: ":" + cfg.HealthPort, Handler: mux}
	go func() {
		log.Printf("lake-promote listening on :%s", cfg.HealthPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Println("lake-promote: consuming", cfg.KafkaTopicBronze)
		if err := consumer.ConsumeBronzeWritten(ctx, func(ctx context.Context, event kafka.BronzeWritten) error {
			log.Printf("promote: received bronze event source=%s key=%s", event.Source, event.Key)
			result, err := promote.PromoteSource(ctx, s3, cfg.BronzeBucket, cfg.SilverBucket, event.Source, event.Key)
			if err != nil {
				return err
			}
			return producer.PublishSilverWritten(ctx, kafka.SilverWritten{
				Source:    result.Source,
				Bucket:    cfg.SilverBucket,
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
