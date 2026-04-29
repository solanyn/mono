package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

const audioHTTPTimeout = 10 * time.Minute

var audioClient = &http.Client{
	Timeout: audioHTTPTimeout,
	Transport: &http.Transport{
		DisableKeepAlives: true, // mlx-audio (uvicorn) closes connections after multipart POSTs
	},
}

// ProcessSegment mirrors scrib-audio's /v1/audio/process segment shape.
type ProcessSegment struct {
	Speaker   string  `json:"speaker"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Text      string  `json:"text"`
	Uncertain bool    `json:"uncertain,omitempty"`
}

// SpeakerEmbedding pairs a diarisation label with a per-speaker vector (base64
// float32 array). scrib-audio produces one per diarised speaker.
type SpeakerEmbedding struct {
	Speaker   string    `json:"speaker"`
	Embedding []float32 `json:"embedding"`
}

type ProcessResult struct {
	Segments          []DiarizedSegment  `json:"-"`
	RawSegments       []ProcessSegment   `json:"segments"`
	SpeakerEmbeddings []SpeakerEmbedding `json:"speaker_embeddings,omitempty"`
	NumSpeakers       int                `json:"num_speakers"`
	DurationSeconds   float64            `json:"duration_seconds"`
	TranscriptText    string             `json:"transcript_text"`
}

// processAudio calls scrib-audio's single-shot pipeline endpoint. Streams the
// request body so we don't buffer another copy of the WAV beyond what the
// multipart writer needs.
func (s *Server) processAudio(ctx context.Context, audioData io.Reader, filename string, threshold string, mergeGap string) (*ProcessResult, error) {
	url := s.cfg.AudioProcessURL
	if url == "" {
		return nil, fmt.Errorf("audio process URL not configured")
	}

	body, ct, err := multipartFromReader(audioData, filename, map[string]string{
		"threshold":          threshold,
		"merge_gap":          mergeGap,
		"include_embeddings": "true",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url+"/v1/audio/process", body)
	if err != nil {
		return nil, fmt.Errorf("process request: %w", err)
	}
	req.Header.Set("Content-Type", ct)

	resp, err := audioClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("process request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("process %d: %s", resp.StatusCode, b)
	}

	var result ProcessResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("process decode: %w", err)
	}
	// Promote raw segments to DiarizedSegment (SpeakerID resolved later).
	result.Segments = make([]DiarizedSegment, 0, len(result.RawSegments))
	for _, rs := range result.RawSegments {
		result.Segments = append(result.Segments, DiarizedSegment{
			Speaker:   rs.Speaker,
			Start:     rs.Start,
			End:       rs.End,
			Text:      rs.Text,
			Uncertain: rs.Uncertain,
		})
	}
	return &result, nil
}

func multipartFromReader(r io.Reader, filename string, fields map[string]string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}

	fw, err := w.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &buf, w.FormDataContentType(), nil
}
