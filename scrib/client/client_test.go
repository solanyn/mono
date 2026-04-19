package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSummarizeURL(t *testing.T) {
	tests := []struct {
		name       string
		suffix     string
		wantPath   string
	}{
		{
			name:     "standard gateway URL",
			suffix:   "/v1/opus",
			wantPath: "/v1/opus/chat/completions",
		},
		{
			name:     "trailing slash stripped",
			suffix:   "/v1/opus/",
			wantPath: "/v1/opus/chat/completions",
		},
		{
			name:     "bare server URL",
			suffix:   "",
			wantPath: "/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				json.NewEncoder(w).Encode(map[string]interface{}{
					"choices": []map[string]interface{}{
						{"message": map[string]string{"content": "ok"}},
					},
				})
			}))
			defer srv.Close()

			c := &Client{
				GatewayURL: srv.URL + tt.suffix,
				HTTPClient: srv.Client(),
			}

			_, err := c.Summarize(context.Background(), "test transcript", "standup")
			if err != nil {
				t.Fatalf("Summarize() error: %v", err)
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}

func TestChunkTranscriptShort(t *testing.T) {
	transcript := "**SPEAKER_0** (0:00): Hello\n**SPEAKER_1** (0:05): Hi there"
	chunks := chunkTranscript(transcript, 100000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != transcript {
		t.Errorf("chunk content mismatch")
	}
}

func TestChunkTranscriptLongSplitsByTurn(t *testing.T) {
	turn1 := "**SPEAKER_0** (0:00): " + strings.Repeat("word ", 100)
	turn2 := "**SPEAKER_1** (1:00): " + strings.Repeat("talk ", 100)
	turn3 := "**SPEAKER_0** (2:00): " + strings.Repeat("more ", 100)
	transcript := turn1 + "\n" + turn2 + "\n" + turn3

	chunks := chunkTranscript(transcript, len(turn1)+len(turn2)+10)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) == 0 {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestChunkTranscriptMassiveTurnSplitsBySentence(t *testing.T) {
	turn := "**SPEAKER_0** (0:00): " + strings.Repeat("This is a sentence. ", 200)
	chunks := chunkTranscript(turn, 500)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for massive turn, got %d", len(chunks))
	}
	rejoined := strings.Join(chunks, "")
	if rejoined != turn {
		t.Errorf("rejoined chunks don't match original")
	}
}

func TestChunkTranscriptMergeSummary(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "summary " + strings.Repeat("x", 10)}},
			},
		})
	}))
	defer srv.Close()

	turn1 := "**SPEAKER_0** (0:00): " + strings.Repeat("word ", 20000)
	turn2 := "**SPEAKER_1** (5:00): " + strings.Repeat("talk ", 20000)
	transcript := turn1 + "\n" + turn2

	c := &Client{
		GatewayURL: srv.URL,
		HTTPClient: srv.Client(),
	}

	_, err := c.Summarize(context.Background(), transcript, "standup")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 LLM calls for chunked transcript, got %d", callCount)
	}
}

func TestSummarizeCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	c := &Client{GatewayURL: srv.URL, HTTPClient: srv.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Summarize(ctx, "test", "standup")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSummarizeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	c := &Client{GatewayURL: srv.URL, HTTPClient: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.Summarize(ctx, "test", "standup")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestSummarizeConfigModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{GatewayURL: srv.URL, HTTPClient: srv.Client(), SummaryModel: "gpt-4o"}
	_, err := c.Summarize(context.Background(), "test", "standup")
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "gpt-4o" {
		t.Errorf("model = %q, want %q", gotModel, "gpt-4o")
	}
}

func TestSummarizeDefaultModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{GatewayURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Summarize(context.Background(), "test", "standup")
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "auto" {
		t.Errorf("model = %q, want %q", gotModel, "auto")
	}
}

func TestSummarizeURLNoDoubleSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{
		GatewayURL: srv.URL + "/v1/opus/",
		HTTPClient: srv.Client(),
	}

	_, err := c.Summarize(context.Background(), "test", "standup")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}

	if strings.Contains(gotPath, "//") {
		t.Errorf("double slash in path: %q", gotPath)
	}
}
