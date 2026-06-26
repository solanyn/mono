package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// analyzeServer returns a fake Presidio that always responds with the given
// detections, regardless of the analyzed text.
func analyzeServer(t *testing.T, results []analyzeResult) *presidioClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(results)
	}))
	t.Cleanup(srv.Close)
	return newPresidioClient(Config{PresidioURL: srv.URL, Language: "en", ScoreThreshold: 0.5})
}

// TestScrubOverlappingPrefersDominantSpan reproduces the real Presidio output
// for an email: EMAIL_ADDRESS covering the whole address plus a narrower URL
// span over the domain. The dominant (wider, earlier) EMAIL_ADDRESS must win so
// the local part ("john@") is not leaked.
func TestScrubOverlappingPrefersDominantSpan(t *testing.T) {
	// Offsets into the text below.
	text := "Jane Doe, email me at john@example.com"
	//       PERSON [0:8] ......................^email [22:38]
	//                                            URL [27:38] (domain only)
	c := analyzeServer(t, []analyzeResult{
		{EntityType: "PERSON", Start: 0, End: 8, Score: 0.85},
		{EntityType: "EMAIL_ADDRESS", Start: 22, End: 38, Score: 1.0},
		{EntityType: "URL", Start: 27, End: 38, Score: 0.5},
	})

	got, changed, err := scrubText(context.Background(), c, text)
	if err != nil || !changed {
		t.Fatalf("err=%v changed=%v", err, changed)
	}

	want := "[PERSON], email me at [EMAIL]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if strings.Contains(got, "john@") {
		t.Fatalf("local part leaked: %q", got)
	}
	if strings.Contains(got, "[URL]") {
		t.Fatalf("masked narrower URL instead of dominant EMAIL_ADDRESS: %q", got)
	}
}

// TestScrubNonOverlappingAdjacent ensures adjacent (touching) spans are both
// masked — an end offset equal to the next start is not an overlap.
func TestScrubNonOverlappingAdjacent(t *testing.T) {
	text := "abcd" // [0:2] and [2:4]
	c := analyzeServer(t, []analyzeResult{
		{EntityType: "PERSON", Start: 0, End: 2, Score: 0.9},
		{EntityType: "US_SSN", Start: 2, End: 4, Score: 0.9},
	})
	got, _, err := scrubText(context.Background(), c, text)
	if err != nil {
		t.Fatal(err)
	}
	if got != "[PERSON][SSN]" {
		t.Fatalf("got %q want %q", got, "[PERSON][SSN]")
	}
}
