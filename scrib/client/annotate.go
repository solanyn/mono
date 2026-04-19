// Package client - annotate.go provides the annotation pipeline.
package client

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiarizedSegment is a transcript segment attributed to a speaker.
type DiarizedSegment struct {
	Speaker   string  `json:"speaker"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Text      string  `json:"text"`
	Uncertain bool    `json:"uncertain,omitempty"`
}

// AnnotateResult holds the full annotated meeting output.
type AnnotateResult struct {
	Segments      []DiarizedSegment
	Summary       string
	SummaryErr    error
	RawVAD        *VADResult
	RawTranscript *TranscriptResult
	Duration      time.Duration
}

// Annotate runs the full pipeline: VAD + STT (concurrent) → merge → summarize.
// If VAD or STT fails, returns an error. If summary fails, the result is still
// returned with SummaryErr set so callers can persist segments before retrying.
func (c *Client) Annotate(ctx context.Context, audioPath string, threshold float64, template string) (*AnnotateResult, error) {
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
		vadResult, vadErr = c.VAD(ctx, audioPath, threshold)
	}()
	go func() {
		defer wg.Done()
		sttResult, sttErr = c.Transcribe(ctx, audioPath)
	}()
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if vadErr != nil {
		return nil, fmt.Errorf("vad: %w", vadErr)
	}
	if sttErr != nil {
		return nil, fmt.Errorf("stt: %w", sttErr)
	}

	segments := alignSpeakersToWords(vadResult, sttResult)
	dur := time.Duration(vadResult.DurationSeconds * float64(time.Second))

	result := &AnnotateResult{
		Segments:      segments,
		RawVAD:        vadResult,
		RawTranscript: sttResult,
		Duration:      dur,
	}

	transcript := formatTranscript(segments)
	summary, err := c.Summarize(ctx, transcript, template)
	if err != nil {
		result.SummaryErr = fmt.Errorf("summarize: %w", err)
		return result, nil
	}
	result.Summary = summary

	return result, nil
}

// alignSpeakersToWords assigns each transcript word to a speaker based on
// time overlap with VAD segments.
func alignSpeakersToWords(vad *VADResult, stt *TranscriptResult) []DiarizedSegment {
	if len(vad.Segments) == 0 || len(stt.Words) == 0 {
		// Fallback: no diarisation, return full text as single segment
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

	// Merge consecutive words with same speaker into segments
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
				Speaker: tagged[i].speaker,
				Start:   tagged[i].start,
				End:     tagged[i].end,
				Text:    tagged[i].word,
			}
		}
	}
	segments = append(segments, cur)

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Start < segments[j].Start
	})

	return segments
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
