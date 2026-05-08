package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/storage"
	"github.com/solanyn/mono/line/internal/telemetry"
)

const maxFramesPerLap = 30000

type session struct {
	id        string
	carID     int32
	startTime time.Time
	laps      map[int16][]storage.TelemetryRow
	lastLap   int16
	lapCount  int
}

type lapWrittenEvent struct {
	SessionID string `json:"session_id"`
	LapNumber int    `json:"lap_number"`
	S3Key     string `json:"s3_key"`
	CarCode   int32  `json:"car_code"`
	Frames    int    `json:"frames"`
}

type sessionCompleteEvent struct {
	SessionID string `json:"session_id"`
	CarCode   int32  `json:"car_code"`
	TrackName string `json:"track_name"`
	LapCount  int    `json:"lap_count"`
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
	lapTopic := envOrDefault("KAFKA_LAP_TOPIC", "line.lap.written")
	sessionTopic := envOrDefault("KAFKA_SESSION_TOPIC", "line.session.complete")
	databaseURL := envOrDefault("DATABASE_URL", "")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	s3Client := storage.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3Region, s3Bucket)

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
		slog.Warn("DATABASE_URL not set, session metadata will not be persisted")
	}

	consumer, err := kafka.NewConsumer(brokers, group, topic)
	if err != nil {
		slog.Error("failed to create consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		slog.Error("failed to create producer", "err", err)
		os.Exit(1)
	}
	defer producer.Close()

	slog.Info("assembler started", "brokers", brokers, "topic", topic, "group", group)

	var sess *session
	var idleTimeout *time.Timer
	var totalFrames int64
	var totalLapsFlushed int64
	var totalSessionsClosed int64

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var bufferedFrames int
				var bufferedLaps int
				if sess != nil {
					for _, rows := range sess.laps {
						bufferedFrames += len(rows)
					}
					bufferedLaps = len(sess.laps)
				}
				slog.Info("assembler stats",
					"total_frames", totalFrames,
					"total_laps_flushed", totalLapsFlushed,
					"total_sessions", totalSessionsClosed,
					"buffered_frames", bufferedFrames,
					"buffered_laps", bufferedLaps,
				)
			}
		}
	}()

	resetIdle := func() {
		if idleTimeout != nil {
			idleTimeout.Stop()
		}
		idleTimeout = time.AfterFunc(30*time.Second, func() {
			if sess != nil {
				slog.Info("session idle timeout, flushing remaining laps", "session", sess.id)
				flushSession(ctx, sess, s3Client, producer, lapTopic, database)
				publishSessionComplete(ctx, producer, sessionTopic, sess, database)
				sess = nil
			}
		})
	}

	err = consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
		if len(record.Value) < telemetry.EncodedFrameSize {
			slog.Debug("skipping undersized record", "size", len(record.Value), "offset", record.Offset)
			return nil
		}

		frame := telemetry.DecodeFrame(record.Value)
		totalFrames++

		if sess == nil || frame.CarID != sess.carID {
			if sess != nil {
				slog.Info("session ended (car change)", "session", sess.id, "old_car", sess.carID, "new_car", frame.CarID)
				flushSession(ctx, sess, s3Client, producer, lapTopic, database)
				publishSessionComplete(ctx, producer, sessionTopic, sess, database)
				totalSessionsClosed++
			}
			sess = &session{
				id:        uuid.NewString()[:8],
				carID:     frame.CarID,
				startTime: time.Now(),
				laps:      make(map[int16][]storage.TelemetryRow),
				lastLap:   frame.CurrentLap,
			}
			slog.Info("new session", "id", sess.id, "car_id", frame.CarID)
			if database != nil {
				if err := database.CreateSession(ctx, &db.Session{
					ID:        sess.id,
					CarCode:   sess.carID,
					StartedAt: sess.startTime,
				}); err != nil {
					slog.Error("db create session", "err", err)
				}
			}
		}

		resetIdle()

		row := frame.ToRow()
		sess.laps[frame.CurrentLap] = append(sess.laps[frame.CurrentLap], row)

		if len(sess.laps[frame.CurrentLap]) >= maxFramesPerLap {
			slog.Warn("lap frame cap reached, force flushing", "session", sess.id, "lap", frame.CurrentLap, "frames", maxFramesPerLap)
			if err := flushLap(ctx, sess, frame.CurrentLap, s3Client, producer, lapTopic, database); err != nil {
				slog.Error("force flush lap failed", "lap", frame.CurrentLap, "err", err)
			}
			totalLapsFlushed++
		}

		if frame.CurrentLap > sess.lastLap {
			if err := flushLap(ctx, sess, sess.lastLap, s3Client, producer, lapTopic, database); err != nil {
				slog.Error("flush lap failed", "lap", sess.lastLap, "err", err)
			}
			totalLapsFlushed++
			sess.lastLap = frame.CurrentLap
		}

		return nil
	})

	if err != nil && ctx.Err() == nil {
		slog.Error("consumer error", "err", err)
		os.Exit(1)
	}

	if sess != nil {
		flushSession(context.Background(), sess, s3Client, producer, lapTopic, database)
		publishSessionComplete(context.Background(), producer, sessionTopic, sess, database)
		totalSessionsClosed++
	}
	slog.Info("assembler stopped", "total_frames", totalFrames, "total_laps", totalLapsFlushed, "total_sessions", totalSessionsClosed)
}

func flushLap(ctx context.Context, sess *session, lapNum int16, s3Client *storage.S3Client, producer *kafka.Producer, lapTopic string, database *db.DB) error {
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
	sess.lapCount++
	delete(sess.laps, lapNum)

	if database != nil {
		if err := database.InsertLap(ctx, &db.Lap{
			SessionID: sess.id,
			LapNumber: int(lapNum),
			Frames:    len(rows),
			S3Key:     key,
		}); err != nil {
			slog.Error("db insert lap", "session", sess.id, "lap", lapNum, "err", err)
		}
	}

	event := lapWrittenEvent{
		SessionID: sess.id,
		LapNumber: int(lapNum),
		S3Key:     key,
		CarCode:   sess.carID,
		Frames:    len(rows),
	}
	eventData, _ := json.Marshal(event)
	producer.ProduceAsync(ctx, lapTopic, sess.id, eventData)

	return nil
}

func flushSession(ctx context.Context, sess *session, s3Client *storage.S3Client, producer *kafka.Producer, lapTopic string, database *db.DB) {
	for lapNum := range sess.laps {
		if err := flushLap(ctx, sess, lapNum, s3Client, producer, lapTopic, database); err != nil {
			slog.Error("flush remaining lap failed", "session", sess.id, "lap", lapNum, "err", err)
		}
	}
}

func publishSessionComplete(ctx context.Context, producer *kafka.Producer, topic string, sess *session, database *db.DB) {
	if database != nil {
		if err := database.EndSession(ctx, sess.id, sess.lapCount, nil); err != nil {
			slog.Error("db end session", "session", sess.id, "err", err)
		}
	}

	event := sessionCompleteEvent{
		SessionID: sess.id,
		CarCode:   sess.carID,
		TrackName: "Unknown",
		LapCount:  sess.lapCount,
	}
	eventData, _ := json.Marshal(event)
	producer.ProduceAsync(ctx, topic, sess.id, eventData)
	slog.Info("published session complete", "session", sess.id, "laps", sess.lapCount)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
