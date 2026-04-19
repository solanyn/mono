// Package client provides HTTP clients for mlx-audio and kgateway endpoints.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultVADTimeout     = 300 * time.Second
	DefaultSTTTimeout     = 300 * time.Second
	DefaultSummaryTimeout = 120 * time.Second
)

// Client wraps HTTP calls to mlx-audio server and kgateway.
type Client struct {
	AudioURL     string
	GatewayURL   string
	APIKey       string
	STTModel     string
	SummaryModel string
	HTTPClient   *http.Client
}

func New(audioURL, gatewayURL, apiKey, sttModel, summaryModel string) *Client {
	if audioURL == "" {
		audioURL = "http://127.0.0.1:8000"
	}
	if gatewayURL == "" {
		gatewayURL = "https://gateway.goyangi.io/v1/opus"
	}
	if sttModel == "" {
		sttModel = "mlx-community/parakeet-tdt-0.6b-v3"
	}
	return &Client{
		AudioURL:     audioURL,
		GatewayURL:   gatewayURL,
		APIKey:       apiKey,
		STTModel:     sttModel,
		SummaryModel: summaryModel,
		HTTPClient:   &http.Client{},
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
func (c *Client) VAD(ctx context.Context, audioPath string, threshold float64) (*VADResult, error) {
	body, ct, err := c.multipartFile(audioPath, map[string]string{
		"threshold": fmt.Sprintf("%.2f", threshold),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultVADTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.AudioURL+"/v1/audio/vad", body)
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
func (c *Client) Transcribe(ctx context.Context, audioPath string) (*TranscriptResult, error) {
	body, ct, err := c.multipartFile(audioPath, map[string]string{
		"model":           c.STTModel,
		"response_format": "verbose_json",
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultSTTTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.AudioURL+"/v1/audio/transcriptions", body)
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

const defaultMaxChunkChars = 100000

// Summarize sends a diarised transcript to the LLM for meeting notes.
// Long transcripts are split into chunks by speaker turn boundaries,
// summarized individually, then merged in a final pass.
func (c *Client) Summarize(ctx context.Context, transcript string, template string) (string, error) {
	chunks := chunkTranscript(transcript, defaultMaxChunkChars)
	if len(chunks) == 1 {
		return c.summarizeOnce(ctx, buildSummaryPrompt(chunks[0], template))
	}

	chunkSummaries := make([]string, len(chunks))
	for i, chunk := range chunks {
		summary, err := c.summarizeOnce(ctx, buildSummaryPrompt(chunk, template))
		if err != nil {
			return "", fmt.Errorf("summarize chunk %d/%d: %w", i+1, len(chunks), err)
		}
		chunkSummaries[i] = summary
	}

	mergePrompt := buildMergePrompt(chunkSummaries, template)
	return c.summarizeOnce(ctx, mergePrompt)
}

func (c *Client) summarizeOnce(ctx context.Context, prompt string) (string, error) {
	model := c.SummaryModel
	if model == "" {
		model = "auto"
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	ctx, cancel := context.WithTimeout(ctx, DefaultSummaryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.GatewayURL, "/")+"/chat/completions", bytes.NewReader(reqBody))
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

// chunkTranscript splits a transcript into chunks of at most maxChars,
// breaking at speaker turn boundaries (lines starting with "**").
// If a single turn exceeds maxChars, it is split at sentence boundaries.
func chunkTranscript(transcript string, maxChars int) []string {
	if len(transcript) <= maxChars {
		return []string{transcript}
	}

	lines := strings.Split(transcript, "\n")
	var turns []string
	var cur strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "**") && cur.Len() > 0 {
			turns = append(turns, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		turns = append(turns, cur.String())
	}

	var chunks []string
	var chunk strings.Builder
	for _, turn := range turns {
		if len(turn) > maxChars {
			if chunk.Len() > 0 {
				chunks = append(chunks, chunk.String())
				chunk.Reset()
			}
			chunks = append(chunks, splitTurnBySentence(turn, maxChars)...)
			continue
		}
		if chunk.Len() > 0 && chunk.Len()+1+len(turn) > maxChars {
			chunks = append(chunks, chunk.String())
			chunk.Reset()
		}
		if chunk.Len() > 0 {
			chunk.WriteByte('\n')
		}
		chunk.WriteString(turn)
	}
	if chunk.Len() > 0 {
		chunks = append(chunks, chunk.String())
	}

	return chunks
}

func splitTurnBySentence(turn string, maxChars int) []string {
	sentences := splitSentences(turn)
	var chunks []string
	var chunk strings.Builder
	for _, s := range sentences {
		if chunk.Len() > 0 && chunk.Len()+len(s) > maxChars {
			chunks = append(chunks, chunk.String())
			chunk.Reset()
		}
		chunk.WriteString(s)
	}
	if chunk.Len() > 0 {
		chunks = append(chunks, chunk.String())
	}
	return chunks
}

func splitSentences(text string) []string {
	var sentences []string
	var cur strings.Builder
	for i, r := range text {
		cur.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && i+1 < len(text) && text[i+1] == ' ' {
			sentences = append(sentences, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		sentences = append(sentences, cur.String())
	}
	return sentences
}

func buildMergePrompt(chunkSummaries []string, template string) string {
	var sb strings.Builder
	sb.WriteString("You are a meeting notes assistant. Below are partial summaries of consecutive sections of a long meeting. ")
	sb.WriteString("Merge them into a single coherent set of meeting notes.\n\n")
	fmt.Fprintf(&sb, "Template: %s\n\n", template)
	sb.WriteString("Output format:\n## Summary\n(2-3 sentences)\n\n## Decisions\n- (bullet points)\n\n## Action Items\n- [ ] Owner: task description\n\n## Transcript\n(cleaned up transcript with speaker labels)\n\n---\n")
	for i, s := range chunkSummaries {
		fmt.Fprintf(&sb, "\n### Section %d\n%s\n", i+1, s)
	}
	return sb.String()
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
