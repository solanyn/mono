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

const (
	vadTimeout = 300 * time.Second
	sttTimeout = 300 * time.Second
)

type VADSegment struct {
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}

type VADResult struct {
	Segments        []VADSegment `json:"segments"`
	NumSpeakers     int          `json:"num_speakers"`
	DurationSeconds float64      `json:"duration_seconds"`
}

type TranscriptWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type TranscriptResult struct {
	Text  string           `json:"text"`
	Words []TranscriptWord `json:"words,omitempty"`
}

func (s *Server) vad(ctx context.Context, audioData io.Reader, filename string, threshold string) (*VADResult, error) {
	body, ct, err := multipartFromReader(audioData, filename, map[string]string{
		"threshold": threshold,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, vadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.AudioServiceURL+"/v1/audio/vad", body)
	if err != nil {
		return nil, fmt.Errorf("vad request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vad request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vad %d: %s", resp.StatusCode, b)
	}

	var result VADResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("vad decode: %w", err)
	}
	return &result, nil
}

func (s *Server) transcribe(ctx context.Context, audioData io.Reader, filename string) (*TranscriptResult, error) {
	body, ct, err := multipartFromReader(audioData, filename, map[string]string{
		"model":           s.cfg.STTModel,
		"response_format": "verbose_json",
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, sttTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.AudioServiceURL+"/v1/audio/transcriptions", body)
	if err != nil {
		return nil, fmt.Errorf("transcribe request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcribe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcribe %d: %s", resp.StatusCode, b)
	}

	var result TranscriptResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("transcribe decode: %w", err)
	}
	return &result, nil
}

func multipartFromReader(r io.Reader, filename string, fields map[string]string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		w.WriteField(k, v)
	}

	fw, err := w.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, "", err
	}
	w.Close()

	return &buf, w.FormDataContentType(), nil
}
