package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type DiarizedSegment struct {
	Speaker   string  `json:"speaker"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Text      string  `json:"text"`
	Uncertain bool    `json:"uncertain,omitempty"`
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

	var meetingID int
	var blobKey sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, audio_blob_key FROM meetings WHERE uuid = $1`, meetingUUID,
	).Scan(&meetingID, &blobKey)
	if err != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("lookup meeting: %w", err))
		return
	}
	if !blobKey.Valid || blobKey.String == "" {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("no audio uploaded"))
		return
	}

	if _, err := s.db.ExecContext(ctx, `UPDATE meetings SET status = 'processing', error = NULL WHERE uuid = $1`, meetingUUID); err != nil {
		log.Printf("mark processing %s: %v", meetingUUID, err)
	}

	var (
		segments    []DiarizedSegment
		numSpeakers int
		duration    float64
	)

	if s.cfg.AudioProcessURL != "" {
		segments, numSpeakers, duration, err = s.runPipelineProcess(ctx, blobKey.String)
	} else {
		segments, numSpeakers, duration, err = s.runPipelineLegacy(ctx, blobKey.String)
	}
	if err != nil {
		s.setMeetingError(ctx, meetingUUID, err)
		return
	}

	if err := s.writeMeetingResults(ctx, meetingID, meetingUUID, segments, numSpeakers, duration); err != nil {
		s.setMeetingError(ctx, meetingUUID, err)
		return
	}

	log.Printf("processed meeting %s: %d segments, %d speakers", meetingUUID, len(segments), numSpeakers)
}

// runPipelineProcess streams the audio from S3 into scrib-audio's single-shot
// /v1/audio/process endpoint and returns aligned diarised segments directly.
func (s *Server) runPipelineProcess(ctx context.Context, blobKey string) ([]DiarizedSegment, int, float64, error) {
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
		return nil, 0, 0, fmt.Errorf("process: %w", err)
	}

	segs := make([]DiarizedSegment, 0, len(result.Segments))
	for _, s := range result.Segments {
		segs = append(segs, DiarizedSegment{
			Speaker:   s.Speaker,
			Start:     s.Start,
			End:       s.End,
			Text:      s.Text,
			Uncertain: s.Uncertain,
		})
	}
	return segs, result.NumSpeakers, result.DurationSeconds, nil
}

// runPipelineLegacy is the pre-scrib-audio flow: VAD + STT + in-Go align.
// Retained so the server still functions when AudioProcessURL is unset (eg.
// talking to bare mlx-audio). New deployments should prefer AudioProcessURL.
func (s *Server) runPipelineLegacy(ctx context.Context, blobKey string) ([]DiarizedSegment, int, float64, error) {
	obj, err := s.s3.GetObject(ctx, s.cfg.S3Bucket, blobKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("s3 get: %w", err)
	}
	audioBytes, err := io.ReadAll(obj)
	obj.Close()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("s3 read: %w", err)
	}

	var vadResult *VADResult
	if err := retry(ctx, 3, 2*time.Second, "vad", func(ctx context.Context) error {
		r, err := s.vadChunked(ctx, audioBytes, blobKey, s.cfg.VADThreshold)
		if err != nil {
			return err
		}
		vadResult = r
		return nil
	}); err != nil {
		return nil, 0, 0, fmt.Errorf("vad: %w", err)
	}

	var sttResult *TranscriptResult
	if err := retry(ctx, 3, 2*time.Second, "stt", func(ctx context.Context) error {
		r, err := s.transcribe(ctx, bytes.NewReader(audioBytes), blobKey)
		if err != nil {
			return err
		}
		sttResult = r
		return nil
	}); err != nil {
		return nil, 0, 0, fmt.Errorf("stt: %w", err)
	}

	segs := alignSpeakersToWords(vadResult, sttResult)
	return segs, vadResult.NumSpeakers, vadResult.DurationSeconds, nil
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
			`INSERT INTO segments (uuid, meeting_id, speaker_label, start_s, end_s, text)
			 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5)`,
			meetingID, seg.Speaker, seg.Start, seg.End, seg.Text,
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

func alignSpeakersToWords(vad *VADResult, stt *TranscriptResult) []DiarizedSegment {
	if len(vad.Segments) == 0 || len(stt.Words) == 0 {
		return []DiarizedSegment{{
			Speaker: "SPEAKER_0",
			Start:   0,
			End:     vad.DurationSeconds,
			Text:    stt.Text,
		}}
	}

	type taggedWord struct {
		word      string
		start     float64
		end       float64
		speaker   string
		uncertain bool
	}

	tagged := make([]taggedWord, len(stt.Words))
	for i, w := range stt.Words {
		tagged[i] = taggedWord{word: w.Word, start: w.Start, end: w.End}
		wordDur := w.End - w.Start

		bestOverlap := 0.0
		bestSpeaker := "UNKNOWN"
		for _, seg := range vad.Segments {
			overlapStart := max(w.Start, seg.Start)
			overlapEnd := min(w.End, seg.End)
			if overlapEnd > overlapStart {
				overlap := overlapEnd - overlapStart
				if overlap > bestOverlap {
					bestOverlap = overlap
					bestSpeaker = seg.Speaker
				}
			}
		}

		if bestSpeaker == "UNKNOWN" {
			mid := (w.Start + w.End) / 2
			minDist := 999999.0
			for _, seg := range vad.Segments {
				d := min(abs(mid-seg.Start), abs(mid-seg.End))
				if d < minDist {
					minDist = d
					bestSpeaker = seg.Speaker
				}
			}
			tagged[i].uncertain = true
		} else if wordDur > 0 && bestOverlap/wordDur < 0.5 {
			tagged[i].uncertain = true
		}

		tagged[i].speaker = bestSpeaker
	}

	var segments []DiarizedSegment
	if len(tagged) == 0 {
		return segments
	}

	cur := DiarizedSegment{
		Speaker:   tagged[0].speaker,
		Start:     tagged[0].start,
		End:       tagged[0].end,
		Text:      tagged[0].word,
		Uncertain: tagged[0].uncertain,
	}

	for i := 1; i < len(tagged); i++ {
		if tagged[i].speaker == cur.Speaker {
			cur.End = tagged[i].end
			cur.Text += " " + tagged[i].word
			if tagged[i].uncertain {
				cur.Uncertain = true
			}
		} else {
			segments = append(segments, cur)
			cur = DiarizedSegment{
				Speaker:   tagged[i].speaker,
				Start:     tagged[i].start,
				End:       tagged[i].end,
				Text:      tagged[i].word,
				Uncertain: tagged[i].uncertain,
			}
		}
	}
	segments = append(segments, cur)

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Start < segments[j].Start
	})

	return segments
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
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
