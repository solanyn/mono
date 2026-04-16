// Package client provides HTTP clients for mlx-audio and kgateway endpoints.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// Client wraps HTTP calls to mlx-audio server and kgateway.
type Client struct {
	AudioURL   string // mlx-audio server (default http://127.0.0.1:8000)
	GatewayURL string // kgateway (default https://gateway.goyangi.io)
	APIKey     string // optional Bearer token for authenticated endpoints
	STTModel   string // STT model name (default mlx-community/parakeet-tdt-0.6b-v3)
	HTTPClient *http.Client
}

func New(audioURL, gatewayURL, apiKey, sttModel string) *Client {
	if audioURL == "" {
		audioURL = "http://127.0.0.1:8000"
	}
	if gatewayURL == "" {
		gatewayURL = "https://gateway.goyangi.io"
	}
	if sttModel == "" {
		sttModel = "mlx-community/parakeet-tdt-0.6b-v3"
	}
	return &Client{
		AudioURL:   audioURL,
		GatewayURL: gatewayURL,
		APIKey:     apiKey,
		STTModel:   sttModel,
		HTTPClient: &http.Client{},
	}
}

// do executes an HTTP request, attaching the API key if configured.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	return c.HTTPClient.Do(req)
}

// VADSegment is a speaker segment from Sortformer.
type VADSegment struct {
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}

// VADResult is the response from /v1/audio/vad.
type VADResult struct {
	Segments        []VADSegment `json:"segments"`
	NumSpeakers     int          `json:"num_speakers"`
	DurationSeconds float64      `json:"duration_seconds"`
}

// VAD runs speaker diarisation on an audio file.
func (c *Client) VAD(audioPath string, threshold float64) (*VADResult, error) {
	body, ct, err := c.multipartFile(audioPath, map[string]string{
		"threshold": fmt.Sprintf("%.2f", threshold),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.AudioURL+"/v1/audio/vad", body)
	if err != nil {
		return nil, fmt.Errorf("vad request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := c.do(req)
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

// TranscriptWord is a word with timestamp from Parakeet.
type TranscriptWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// TranscriptResult is the response from /v1/audio/transcriptions.
type TranscriptResult struct {
	Text  string           `json:"text"`
	Words []TranscriptWord `json:"words,omitempty"`
}

// Transcribe runs STT on an audio file.
func (c *Client) Transcribe(audioPath string) (*TranscriptResult, error) {
	body, ct, err := c.multipartFile(audioPath, map[string]string{
		"model":           c.STTModel,
		"response_format": "verbose_json",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.AudioURL+"/v1/audio/transcriptions", body)
	if err != nil {
		return nil, fmt.Errorf("transcribe request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := c.do(req)
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

// Summarize sends a diarised transcript to the LLM for meeting notes.
func (c *Client) Summarize(transcript string, template string) (string, error) {
	prompt := buildSummaryPrompt(transcript, template)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": "auto",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, err := http.NewRequest("POST", c.GatewayURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("summarize request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("summarize request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("summarize %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("summarize decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("summarize: no choices returned")
	}
	return result.Choices[0].Message.Content, nil
}

func buildSummaryPrompt(transcript, template string) string {
	return fmt.Sprintf(`You are a meeting notes assistant. Given this diarised meeting transcript, produce structured notes.

Template: %s

Output format:
## Summary
(2-3 sentences)

## Decisions
- (bullet points)

## Action Items
- [ ] Owner: task description

## Transcript
(cleaned up transcript with speaker labels)

---
Transcript:
%s`, template, transcript)
}

func (c *Client) multipartFile(path string, fields map[string]string) (io.Reader, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		w.WriteField(k, v)
	}

	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, "", err
	}
	w.Close()

	return &buf, w.FormDataContentType(), nil
}
