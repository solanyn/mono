package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/data"
	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/kafka"
	"github.com/solanyn/mono/line/internal/storage"
	"github.com/solanyn/mono/line/internal/telemetry"
	"github.com/solanyn/mono/line/internal/tracks"
)

const maxFramesPerLap = 30000
const maxConcurrentLaps = 5

type session struct {
	id        string
	carID     int32
	startTime time.Time
	laps      map[int16][]storage.TelemetryRow
	lastLap   int16
	lapCount  int
	trackID   string
	trackName string
	bestLapMs int32
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
	TrackID   string `json:"track_id"`
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

	trackDB := tracks.NewDatabase(data.TracksJSON)

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

	var mu sync.Mutex
	var sess *session
	var idleTimeout *time.Timer
	var totalFrames atomic.Int64
	var totalLapsFlushed atomic.Int64
	var totalSessionsClosed atomic.Int64

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
				mu.Lock()
				if sess != nil {
					for _, rows := range sess.laps {
						bufferedFrames += len(rows)
					}
					bufferedLaps = len(sess.laps)
				}
				mu.Unlock()
				slog.Info("assembler stats",
					"total_frames", totalFrames.Load(),
					"total_laps_flushed", totalLapsFlushed.Load(),
					"total_sessions", totalSessionsClosed.Load(),
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
			mu.Lock()
			defer mu.Unlock()
			if sess != nil {
				slog.Info("session idle timeout, flushing remaining laps", "session", sess.id)
				flushSession(ctx, sess, s3Client, producer, lapTopic, database, trackDB)
				publishSessionComplete(ctx, producer, sessionTopic, sess, database)
				totalSessionsClosed.Add(1)
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
		totalFrames.Add(1)

		mu.Lock()
		defer mu.Unlock()

		if sess == nil || frame.CarID != sess.carID {
			if sess != nil {
				slog.Info("session ended (car change)", "session", sess.id, "old_car", sess.carID, "new_car", frame.CarID)
				flushSession(ctx, sess, s3Client, producer, lapTopic, database, trackDB)
				publishSessionComplete(ctx, producer, sessionTopic, sess, database)
				totalSessionsClosed.Add(1)
			}
			sess = &session{
				id:        uuid.NewString()[:16],
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

		if len(sess.laps) > maxConcurrentLaps {
			var oldestLap int16
			var found bool
			for lapNum := range sess.laps {
				if lapNum != frame.CurrentLap && (!found || lapNum < oldestLap) {
					oldestLap = lapNum
					found = true
				}
			}
			if found {
				slog.Warn("too many concurrent lap buffers, flushing oldest", "session", sess.id, "lap", oldestLap, "concurrent", len(sess.laps))
				if err := flushLap(ctx, sess, oldestLap, nil, s3Client, producer, lapTopic, database, trackDB); err != nil {
					slog.Error("flush oldest lap failed", "lap", oldestLap, "err", err)
				}
				totalLapsFlushed.Add(1)
			}
		}

		if len(sess.laps[frame.CurrentLap]) >= maxFramesPerLap {
			slog.Warn("lap frame cap reached, force flushing", "session", sess.id, "lap", frame.CurrentLap, "frames", maxFramesPerLap)
			if err := flushLap(ctx, sess, frame.CurrentLap, nil, s3Client, producer, lapTopic, database, trackDB); err != nil {
				slog.Error("force flush lap failed", "lap", frame.CurrentLap, "err", err)
			}
			totalLapsFlushed.Add(1)
		}

		if frame.CurrentLap > sess.lastLap {
			var lapTimeMs *int32
			if frame.LastLap > 0 {
				lt := frame.LastLap
				lapTimeMs = &lt
			}
			if err := flushLap(ctx, sess, sess.lastLap, lapTimeMs, s3Client, producer, lapTopic, database, trackDB); err != nil {
				slog.Error("flush lap failed", "lap", sess.lastLap, "err", err)
			}
			totalLapsFlushed.Add(1)
			sess.lastLap = frame.CurrentLap
		}

		return nil
	})

	if err != nil && ctx.Err() == nil {
		slog.Error("consumer error", "err", err)
		os.Exit(1)
	}

	if idleTimeout != nil {
		idleTimeout.Stop()
	}
	mu.Lock()
	if sess != nil {
		flushSession(context.Background(), sess, s3Client, producer, lapTopic, database, trackDB)
		publishSessionComplete(context.Background(), producer, sessionTopic, sess, database)
		totalSessionsClosed.Add(1)
	}
	mu.Unlock()
	slog.Info("assembler stopped", "total_frames", totalFrames.Load(), "total_laps", totalLapsFlushed.Load(), "total_sessions", totalSessionsClosed.Load())
}

func flushLap(ctx context.Context, sess *session, lapNum int16, lapTimeMs *int32, s3Client *storage.S3Client, producer *kafka.Producer, lapTopic string, database *db.DB, trackDB *tracks.Database) error {
	rows, ok := sess.laps[lapNum]
	if !ok || len(rows) == 0 {
		return nil
	}

	if sess.trackID == "" && len(rows) >= 100 {
		x := make([]float64, len(rows))
		z := make([]float64, len(rows))
		for i, r := range rows {
			x[i] = float64(r.PosX)
			z[i] = float64(r.PosZ)
		}
		if info := trackDB.Identify(x, z); info != nil {
			sess.trackID = info.TrackID
			sess.trackName = info.Name
			slog.Info("track identified", "session", sess.id, "track_id", info.TrackID, "track_name", info.Name)
			if database != nil {
				trackID := info.TrackID
				trackName := info.Name
				database.UpdateSessionTrack(ctx, sess.id, &trackID, &trackName)
			}
		} else {
			fp := tracks.Fingerprint(x, z, 64)
			sess.trackID = fp
			sess.trackName = "Unknown"
			slog.Info("track not recognized, using fingerprint", "session", sess.id, "fingerprint", fp)
			if database != nil {
				database.UpdateSessionTrack(ctx, sess.id, &fp, &sess.trackName)
			}
		}
	}

	data, err := storage.WriteParquet(rows)
	if err != nil {
		return fmt.Errorf("write parquet: %w", err)
	}

	key, err := s3Client.PutLap(ctx, sess.id, int(lapNum), data)
	if err != nil {
		return fmt.Errorf("put lap: %w", err)
	}

	var topSpeed float32
	for _, r := range rows {
		if r.Speed > topSpeed {
			topSpeed = r.Speed
		}
	}

	slog.Info("flushed lap", "session", sess.id, "lap", lapNum, "frames", len(rows), "key", key, "time_ms", lapTimeMs, "top_speed", topSpeed)
	sess.lapCount++
	if lapTimeMs != nil && (*lapTimeMs < sess.bestLapMs || sess.bestLapMs == 0) {
		sess.bestLapMs = *lapTimeMs
	}
	delete(sess.laps, lapNum)

	if database != nil {
		var ts *float32
		if topSpeed > 0 {
			ts = &topSpeed
		}
		if err := database.InsertLap(ctx, &db.Lap{
			SessionID: sess.id,
			LapNumber: int(lapNum),
			TimeMs:    lapTimeMs,
			Frames:    len(rows),
			TopSpeed:  ts,
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

func flushSession(ctx context.Context, sess *session, s3Client *storage.S3Client, producer *kafka.Producer, lapTopic string, database *db.DB, trackDB *tracks.Database) {
	for lapNum := range sess.laps {
		if err := flushLap(ctx, sess, lapNum, nil, s3Client, producer, lapTopic, database, trackDB); err != nil {
			slog.Error("flush remaining lap failed", "session", sess.id, "lap", lapNum, "err", err)
		}
	}
}

func publishSessionComplete(ctx context.Context, producer *kafka.Producer, topic string, sess *session, database *db.DB) {
	if database != nil {
		var bestLap *int32
		if sess.bestLapMs > 0 {
			bestLap = &sess.bestLapMs
		}
		if err := database.EndSession(ctx, sess.id, sess.lapCount, bestLap); err != nil {
			slog.Error("db end session", "session", sess.id, "err", err)
		}
	}

	event := sessionCompleteEvent{
		SessionID: sess.id,
		CarCode:   sess.carID,
		TrackID:   sess.trackID,
		TrackName: sess.trackName,
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
