package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type DiarizedSegment struct {
	Speaker    string  `json:"speaker"`
	SpeakerID  *int    `json:"speaker_id,omitempty"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Text       string  `json:"text"`
	Uncertain  bool    `json:"uncertain,omitempty"`
	Embeddings []byte  `json:"-"`
}

// retry runs fn up to attempts times with exponential backoff (base, base*2, base*4, ...).
// It aborts immediately if ctx is cancelled.
func retry(ctx context.Context, attempts int, base time.Duration, label string, fn func(context.Context) error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			wait := base << (i - 1)
			log.Printf("%s: attempt %d after %v (prev err: %v)", label, i+1, wait, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if err = fn(ctx); err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return err
}

func (s *Server) processMeeting(parent context.Context, meetingUUID string) {
	// Give each meeting its own derived context so it observes server shutdown
	// but can't be starved on a single hung upstream request.
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	bus := s.bus(meetingUUID)
	defer s.closeBus(meetingUUID)
	bus.publish(Event{Stage: "started", At: time.Now()})

	var meetingID int
	var blobKey sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, audio_blob_key FROM meetings WHERE uuid = $1`, meetingUUID,
	).Scan(&meetingID, &blobKey)
	if err != nil {
		s.failMeeting(ctx, bus, meetingUUID, fmt.Errorf("lookup meeting: %w", err))
		return
	}
	if !blobKey.Valid || blobKey.String == "" {
		s.failMeeting(ctx, bus, meetingUUID, fmt.Errorf("no audio uploaded"))
		return
	}

	if _, err := s.db.ExecContext(ctx, `UPDATE meetings SET status = 'processing', error = NULL WHERE uuid = $1`, meetingUUID); err != nil {
		log.Printf("mark processing %s: %v", meetingUUID, err)
	}

	result, err := s.runPipeline(ctx, bus, blobKey.String)
	if err != nil {
		s.failMeeting(ctx, bus, meetingUUID, err)
		return
	}

	bus.publish(Event{Stage: "matching", At: time.Now()})
	segments := s.matchSpeakers(ctx, meetingID, result.Segments, result.SpeakerEmbeddings)

	if err := s.writeMeetingResults(ctx, meetingID, meetingUUID, segments, result.NumSpeakers, result.DurationSeconds); err != nil {
		s.failMeeting(ctx, bus, meetingUUID, err)
		return
	}

	bus.publish(Event{Stage: "done", At: time.Now(), Detail: fmt.Sprintf("%d segments, %d speakers", len(segments), result.NumSpeakers)})
	log.Printf("processed meeting %s: %d segments, %d speakers", meetingUUID, len(segments), result.NumSpeakers)
}

// runPipeline streams audio from S3 into scrib-audio's single-shot
// /v1/audio/process endpoint. scrib-audio does VAD, STT, alignment, and
// per-speaker embedding; the server just persists.
func (s *Server) runPipeline(ctx context.Context, bus *eventBus, blobKey string) (*ProcessResult, error) {
	if s.cfg.AudioProcessURL == "" {
		return nil, fmt.Errorf("audio process URL not configured")
	}
	bus.publish(Event{Stage: "processing", At: time.Now(), Detail: "calling scrib-audio"})

	var result *ProcessResult
	err := retry(ctx, 3, 2*time.Second, "process", func(ctx context.Context) error {
		obj, err := s.s3.GetObject(ctx, s.cfg.S3Bucket, blobKey, minio.GetObjectOptions{})
		if err != nil {
			return fmt.Errorf("s3 get: %w", err)
		}
		defer obj.Close()

		r, err := s.processAudio(ctx, obj, blobKey, s.cfg.VADThreshold, s.cfg.MergeGap)
		if err != nil {
			return err
		}
		result = r
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("process: %w", err)
	}
	return result, nil
}

func (s *Server) writeMeetingResults(ctx context.Context, meetingID int, meetingUUID string, segments []DiarizedSegment, numSpeakers int, duration float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM segments WHERE meeting_id = $1`, meetingID); err != nil {
		return fmt.Errorf("clear segments: %w", err)
	}

	for _, seg := range segments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO segments (uuid, meeting_id, speaker_id, speaker_label, start_s, end_s, text)
			 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6)`,
			meetingID, seg.SpeakerID, seg.Speaker, seg.Start, seg.End, seg.Text,
		); err != nil {
			return fmt.Errorf("insert segment: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE meetings SET status = 'done', num_speakers = $1, duration_s = $2, processed_at = $3, error = NULL WHERE uuid = $4`,
		numSpeakers, duration, time.Now(), meetingUUID,
	); err != nil {
		return fmt.Errorf("update meeting: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Server) failMeeting(ctx context.Context, bus *eventBus, uuid string, err error) {
	bus.publish(Event{Stage: "error", At: time.Now(), Detail: err.Error()})
	s.setMeetingError(ctx, uuid, err)
}

func (s *Server) setMeetingError(ctx context.Context, uuid string, err error) {
	log.Printf("process error for %s: %v", uuid, err)
	// If ctx is already done, fall back to a short-lived context so we can still
	// persist the failure status.
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	if _, dbErr := s.db.ExecContext(ctx,
		`UPDATE meetings SET status = 'error', error = $1 WHERE uuid = $2`,
		err.Error(), uuid,
	); dbErr != nil {
		log.Printf("persist error for %s: %v", uuid, dbErr)
	}
}

func formatTranscript(segments []DiarizedSegment) string {
	var sb strings.Builder
	for _, seg := range segments {
		mins := int(seg.Start) / 60
		secs := int(seg.Start) % 60
		fmt.Fprintf(&sb, "**%s** (%d:%02d): %s\n", seg.Speaker, mins, secs, seg.Text)
	}
	return sb.String()
}

// sortSegments keeps output stable: segments ordered by start time.
func sortSegments(segs []DiarizedSegment) {
	sort.Slice(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
}
