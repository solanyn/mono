package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/storage"
	"github.com/solanyn/mono/line/internal/telemetry"
)

type session struct {
	id        string
	carID     int32
	startTime time.Time
	laps      map[int16][]storage.TelemetryRow
	lastLap   int16
}

func main() {
	brokers := strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ",")
	topic := envOrDefault("KAFKA_TOPIC", "line.telemetry.raw")
	group := envOrDefault("KAFKA_GROUP", "line.assembler")
	s3Endpoint := envOrDefault("S3_ENDPOINT", "http://localhost:3900")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Region := envOrDefault("S3_REGION", "us-east-1")
	s3Bucket := envOrDefault("S3_BUCKET", "line-bronze")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	s3Client := storage.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3Region, s3Bucket)

	consumer, err := kafka.NewConsumer(brokers, group, topic)
	if err != nil {
		slog.Error("failed to create consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	slog.Info("assembler started", "brokers", brokers, "topic", topic, "group", group)

	var sess *session
	var idleTimeout *time.Timer

	resetIdle := func() {
		if idleTimeout != nil {
			idleTimeout.Stop()
		}
		idleTimeout = time.AfterFunc(30*time.Second, func() {
			if sess != nil {
				slog.Info("session idle timeout, flushing remaining laps", "session", sess.id)
				flushSession(ctx, sess, s3Client)
				sess = nil
			}
		})
	}

	err = consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
		if len(record.Value) < telemetry.EncodedFrameSize {
			return nil
		}

		frame := telemetry.DecodeFrame(record.Value)

		if sess == nil || frame.CarID != sess.carID {
			if sess != nil {
				flushSession(ctx, sess, s3Client)
			}
			sess = &session{
				id:        uuid.NewString()[:8],
				carID:     frame.CarID,
				startTime: time.Now(),
				laps:      make(map[int16][]storage.TelemetryRow),
				lastLap:   frame.CurrentLap,
			}
			slog.Info("new session", "id", sess.id, "car_id", frame.CarID)
		}

		resetIdle()

		row := frame.ToRow()
		sess.laps[frame.CurrentLap] = append(sess.laps[frame.CurrentLap], row)

		if frame.CurrentLap > sess.lastLap {
			if err := flushLap(ctx, sess, sess.lastLap, s3Client); err != nil {
				slog.Error("flush lap failed", "lap", sess.lastLap, "err", err)
			}
			sess.lastLap = frame.CurrentLap
		}

		return nil
	})

	if err != nil && ctx.Err() == nil {
		slog.Error("consumer error", "err", err)
		os.Exit(1)
	}

	if sess != nil {
		flushSession(context.Background(), sess, s3Client)
	}
	slog.Info("assembler stopped")
}

func flushLap(ctx context.Context, sess *session, lapNum int16, s3Client *storage.S3Client) error {
	rows, ok := sess.laps[lapNum]
	if !ok || len(rows) == 0 {
		return nil
	}

	data, err := storage.WriteParquet(rows)
	if err != nil {
		return fmt.Errorf("write parquet: %w", err)
	}

	key, err := s3Client.PutLap(ctx, sess.id, int(lapNum), data)
	if err != nil {
		return fmt.Errorf("put lap: %w", err)
	}

	slog.Info("flushed lap", "session", sess.id, "lap", lapNum, "frames", len(rows), "key", key)
	delete(sess.laps, lapNum)
	return nil
}

func flushSession(ctx context.Context, sess *session, s3Client *storage.S3Client) {
	for lapNum := range sess.laps {
		if err := flushLap(ctx, sess, lapNum, s3Client); err != nil {
			slog.Error("flush remaining lap failed", "session", sess.id, "lap", lapNum, "err", err)
		}
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
