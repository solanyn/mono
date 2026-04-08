// Package client - annotate.go provides the annotation pipeline.
package client

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiarizedSegment is a transcript segment attributed to a speaker.
type DiarizedSegment struct {
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
}

// AnnotateResult holds the full annotated meeting output.
type AnnotateResult struct {
	Segments    []DiarizedSegment
	Summary     string
	RawVAD      *VADResult
	RawTranscript *TranscriptResult
	Duration    time.Duration
}

// Annotate runs the full pipeline: VAD + STT (concurrent) → merge → summarize.
func (c *Client) Annotate(audioPath string, threshold float64, template string) (*AnnotateResult, error) {
	var (
		vadResult *VADResult
		sttResult *TranscriptResult
		vadErr    error
		sttErr    error
		wg        sync.WaitGroup
	)

	// Run VAD and STT concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		vadResult, vadErr = c.VAD(audioPath, threshold)
	}()
	go func() {
		defer wg.Done()
		sttResult, sttErr = c.Transcribe(audioPath)
	}()
	wg.Wait()

	if vadErr != nil {
		return nil, fmt.Errorf("vad: %w", vadErr)
	}
	if sttErr != nil {
		return nil, fmt.Errorf("stt: %w", sttErr)
	}

	// Merge speaker segments with transcript
	segments := alignSpeakersToWords(vadResult, sttResult)

	// Build diarised transcript text
	transcript := formatTranscript(segments)

	// Summarize
	summary, err := c.Summarize(transcript, template)
	if err != nil {
		return nil, fmt.Errorf("summarize: %w", err)
	}

	dur := time.Duration(vadResult.DurationSeconds * float64(time.Second))

	return &AnnotateResult{
		Segments:      segments,
		Summary:       summary,
		RawVAD:        vadResult,
		RawTranscript: sttResult,
		Duration:      dur,
	}, nil
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
		word    string
		start   float64
		end     float64
		speaker string
	}

	tagged := make([]taggedWord, len(stt.Words))
	for i, w := range stt.Words {
		tagged[i] = taggedWord{word: w.Word, start: w.Start, end: w.End}
		mid := (w.Start + w.End) / 2

		// Find best overlapping speaker segment
		bestOverlap := 0.0
		bestSpeaker := "UNKNOWN"
		for _, seg := range vad.Segments {
			overlapStart := max(mid-0.01, seg.Start)
			overlapEnd := min(mid+0.01, seg.End)
			if overlapEnd > overlapStart {
				overlap := overlapEnd - overlapStart
				if overlap > bestOverlap {
					bestOverlap = overlap
					bestSpeaker = seg.Speaker
				}
			}
		}
		// Fallback: find nearest segment
		if bestSpeaker == "UNKNOWN" {
			minDist := 999999.0
			for _, seg := range vad.Segments {
				d := min(abs(mid-seg.Start), abs(mid-seg.End))
				if d < minDist {
					minDist = d
					bestSpeaker = seg.Speaker
				}
			}
		}
		tagged[i].speaker = bestSpeaker
	}

	// Merge consecutive words with same speaker into segments
	var segments []DiarizedSegment
	if len(tagged) == 0 {
		return segments
	}

	cur := DiarizedSegment{
		Speaker: tagged[0].speaker,
		Start:   tagged[0].start,
		End:     tagged[0].end,
		Text:    tagged[0].word,
	}

	for i := 1; i < len(tagged); i++ {
		if tagged[i].speaker == cur.Speaker {
			cur.End = tagged[i].end
			cur.Text += " " + tagged[i].word
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
