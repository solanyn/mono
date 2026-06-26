package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// analyzeResult is a single PII detection returned by Presidio's /analyze
// endpoint. Start and End are offsets into the analyzed text.
type analyzeResult struct {
	EntityType string  `json:"entity_type"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Score      float64 `json:"score"`
}

// presidioClient is a thin HTTP client for the Presidio Analyzer REST API.
type presidioClient struct {
	baseURL        string
	language       string
	scoreThreshold float64
	http           *http.Client
}

// newPresidioClient constructs a client from the given configuration. The
// underlying HTTP client enforces a 5s timeout on every request.
func newPresidioClient(cfg Config) *presidioClient {
	return &presidioClient{
		baseURL:        cfg.PresidioURL,
		language:       cfg.Language,
		scoreThreshold: cfg.ScoreThreshold,
		http:           &http.Client{Timeout: 5 * time.Second},
	}
}

// analyze calls Presidio POST /analyze and returns the detected PII spans.
func (c *presidioClient) analyze(ctx context.Context, text string) ([]analyzeResult, error) {
	// score_threshold MUST be encoded as a JSON number; Presidio rejects a
	// string value with HTTP 400.
	payload, err := json.Marshal(map[string]any{
		"text":            text,
		"language":        c.language,
		"score_threshold": c.scoreThreshold,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("presidio /analyze returned status %d", resp.StatusCode)
	}

	var results []analyzeResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results, nil
}

// health calls Presidio GET /health and reports whether the analyzer is up.
func (c *presidioClient) health(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
