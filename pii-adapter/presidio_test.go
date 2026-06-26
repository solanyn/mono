package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAnalyzeEncodesScoreThresholdAsNumber guards against the regression where
// score_threshold was marshaled as a JSON string ("0.5"), which Presidio
// rejects with HTTP 400. The /analyze request body must encode it as a number.
func TestAnalyzeEncodesScoreThresholdAsNumber(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := newPresidioClient(Config{PresidioURL: srv.URL, Language: "en", ScoreThreshold: 0.5})
	if _, err := c.analyze(context.Background(), "hello"); err != nil {
		t.Fatalf("analyze returned error: %v", err)
	}

	// Decode generically: JSON numbers decode to float64, strings to string.
	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("failed to decode captured body %q: %v", captured, err)
	}

	if _, isString := body["score_threshold"].(string); isString {
		t.Fatalf("score_threshold encoded as JSON string, want number; body=%s", captured)
	}
	num, ok := body["score_threshold"].(float64)
	if !ok {
		t.Fatalf("score_threshold is %T, want float64 (JSON number); body=%s", body["score_threshold"], captured)
	}
	if num != 0.5 {
		t.Fatalf("score_threshold = %v, want 0.5", num)
	}
}

// TestParseFloat covers the SCORE_THRESHOLD parsing helper.
func TestParseFloat(t *testing.T) {
	cases := []struct {
		in   string
		def  float64
		want float64
	}{
		{"", 0.5, 0.5},
		{"0.8", 0.5, 0.8},
		{"not-a-number", 0.5, 0.5},
	}
	for _, tc := range cases {
		if got := parseFloat(tc.in, tc.def); got != tc.want {
			t.Errorf("parseFloat(%q, %v) = %v, want %v", tc.in, tc.def, got, tc.want)
		}
	}
}
