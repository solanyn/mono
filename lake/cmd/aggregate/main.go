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
	"github.com/solanyn/mono/lake/internal/config"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/storage"
)

func main() {
	cfg := config.Load()
	s3 := storage.NewClient(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region)

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	consumer, err := kafka.NewConsumer(brokers, "lake-aggregate", cfg.KafkaTopicSilver)
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

	srv := &http.Server{Addr: ":" + cfg.HealthPort, Handler: mux}
	go func() {
		log.Printf("lake-aggregate listening on :%s", cfg.HealthPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		log.Println("lake-aggregate: consuming", cfg.KafkaTopicSilver)
		if err := consumer.ConsumeSilverWritten(ctx, func(ctx context.Context, event kafka.SilverWritten) error {
			log.Printf("aggregate: received silver event source=%s key=%s", event.Source, event.Key)
			return aggregate.SilverToGold(ctx, s3, cfg.SilverBucket, cfg.GoldBucket, event.Source, event.Key)
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
