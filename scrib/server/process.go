package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
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

func (s *Server) processMeeting(meetingUUID string) {
	ctx := context.Background()

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

	s.db.ExecContext(ctx, `UPDATE meetings SET status = 'processing' WHERE uuid = $1`, meetingUUID)

	obj, err := s.s3.GetObject(ctx, s.cfg.S3Bucket, blobKey.String, minio.GetObjectOptions{})
	if err != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("s3 get: %w", err))
		return
	}
	var audioBuf bytes.Buffer
	if _, err := audioBuf.ReadFrom(obj); err != nil {
		obj.Close()
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("s3 read: %w", err))
		return
	}
	obj.Close()
	audioBytes := audioBuf.Bytes()

	var (
		vadResult *VADResult
		sttResult *TranscriptResult
		vadErr    error
		sttErr    error
		wg        sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		vadResult, vadErr = s.vad(ctx, bytes.NewReader(audioBytes), blobKey.String, s.cfg.VADThreshold)
	}()
	go func() {
		defer wg.Done()
		sttResult, sttErr = s.transcribe(ctx, bytes.NewReader(audioBytes), blobKey.String)
	}()
	wg.Wait()

	if vadErr != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("vad: %w", vadErr))
		return
	}
	if sttErr != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("stt: %w", sttErr))
		return
	}

	segments := alignSpeakersToWords(vadResult, sttResult)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("begin tx: %w", err))
		return
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM segments WHERE meeting_id = $1`, meetingID)

	for _, seg := range segments {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO segments (uuid, meeting_id, speaker_label, start_s, end_s, text)
			 VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5)`,
			meetingID, seg.Speaker, seg.Start, seg.End, seg.Text,
		)
		if err != nil {
			s.setMeetingError(ctx, meetingUUID, fmt.Errorf("insert segment: %w", err))
			return
		}
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE meetings SET status = 'done', num_speakers = $1, duration_s = $2, processed_at = $3, error = NULL WHERE uuid = $4`,
		vadResult.NumSpeakers, vadResult.DurationSeconds, now, meetingUUID,
	)
	if err != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("update meeting: %w", err))
		return
	}

	if err := tx.Commit(); err != nil {
		s.setMeetingError(ctx, meetingUUID, fmt.Errorf("commit: %w", err))
		return
	}

	log.Printf("processed meeting %s: %d segments, %d speakers", meetingUUID, len(segments), vadResult.NumSpeakers)
}

func (s *Server) setMeetingError(ctx context.Context, uuid string, err error) {
	log.Printf("process error for %s: %v", uuid, err)
	s.db.ExecContext(ctx,
		`UPDATE meetings SET status = 'error', error = $1 WHERE uuid = $2`,
		err.Error(), uuid,
	)
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
