package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

			_, err := c.Summarize("test transcript", "standup")
			if err != nil {
				t.Fatalf("Summarize() error: %v", err)
			}

			if gotPath != tt.wantPath {
				t.Errorf("got path %q, want %q", gotPath, tt.wantPath)
			}
		})
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

	_, err := c.Summarize("test", "standup")
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}

	if strings.Contains(gotPath, "//") {
		t.Errorf("double slash in path: %q", gotPath)
	}
}
