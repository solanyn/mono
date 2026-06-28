// Package main implements agentgateway-discovery — an on-demand /v1/models
// aggregator for AgentGateway. Instead of a cronjob refreshing a static
// configmap, this service queries each upstream LLM provider's /v1/models
// endpoint live when the discovery endpoint is called, merges the results
// into a single OpenAI-compatible list, and caches the merged payload in
// memory for a short TTL to avoid hammering upstreams.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	cacheTTL    = 30 * time.Second
	upstreamTTL = 10 * time.Second
	defaultOwn  = "agentgateway"
)

// Provider describes one upstream OpenAI-compatible /v1/models endpoint.
// APIKeyEnv names an environment variable that — when set — supplies the
// Bearer token sent to that provider. An empty APIKeyEnv disables auth.
type Provider struct {
	Name      string
	URL       string
	APIKeyEnv string
}

// providers is the static upstream set. Order is preserved in the merged
// output so the response is stable for clients that key off position.
var providers = []Provider{
	{Name: "mlx", URL: "http://mac.internal:8080/v1/models", APIKeyEnv: ""},
	{Name: "minimax", URL: "https://api.minimaxi.chat/v1/models", APIKeyEnv: "MINIMAX_API_KEY"},
	{Name: "deepseek", URL: "https://api.deepseek.com/v1/models", APIKeyEnv: "DEEPSEEK_API_KEY"},
}

// Model is the OpenAI-compatible model object emitted in the response.
// Tags match OpenAI's /v1/models schema; extra upstream fields are ignored.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
}

// upstreamResponse is what we expect to decode from each provider's /v1/models.
type upstreamResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// outputResponse is the merged payload we serve.
type outputResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// entry is one TTL-cached payload. We store marshalled bytes rather than
// the struct so we don't pay JSON encoding cost on the hot path.
type entry struct {
	bytes     []byte
	expiresAt time.Time
}

// modelCache is a one-slot TTL cache for the merged /v1/models payload.
// Key is intentionally constant — there is exactly one cache entry.
type modelCache struct {
	mu   sync.RWMutex
	data *entry
}

func (c *modelCache) get(now time.Time) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data == nil || !now.Before(c.data.expiresAt) {
		return nil, false
	}
	// hand back a copy so callers can't mutate the cached buffer
	out := make([]byte, len(c.data.bytes))
	copy(out, c.data.bytes)
	return out, true
}

func (c *modelCache) set(b []byte, expiresAt time.Time) {
	stored := make([]byte, len(b))
	copy(stored, b)
	c.mu.Lock()
	c.data = &entry{bytes: stored, expiresAt: expiresAt}
	c.mu.Unlock()
}

// fetchProvider GETs /v1/models from one upstream and returns its models.
// Auth: if the named env var is set, send Authorization: Bearer <value>.
// Otherwise the request goes out without an Authorization header.
func fetchProvider(ctx context.Context, p Provider) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", p.Name, err)
	}
	if p.APIKeyEnv != "" {
		if key := os.Getenv(p.APIKeyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	client := &http.Client{Timeout: upstreamTTL}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("%s: status %d: %s", p.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var ur upstreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", p.Name, err)
	}

	// Fill in fields upstreams often omit so the merged output is uniform.
	for i := range ur.Data {
		if ur.Data[i].Object == "" {
			ur.Data[i].Object = "model"
		}
		if ur.Data[i].OwnedBy == "" {
			ur.Data[i].OwnedBy = defaultOwn
		}
	}
	return ur.Data, nil
}

// mergeModels flattens per-provider slices and deduplicates by id.
// First occurrence wins (preserves provider order from the providers list).
// Empty IDs are skipped. Missing owned_by defaults to "agentgateway".
func mergeModels(slices [][]Model) []Model {
	seen := make(map[string]struct{}, 64)
	out := make([]Model, 0, 64)
	for _, s := range slices {
		for _, m := range s {
			if m.ID == "" {
				continue
			}
			if _, ok := seen[m.ID]; ok {
				continue
			}
			seen[m.ID] = struct{}{}
			if m.OwnedBy == "" {
				m.OwnedBy = defaultOwn
			}
			out = append(out, m)
		}
	}
	return out
}

// aggregate fetches all providers in parallel and merges their models.
// Per-provider failures are logged and skipped — the merged result still
// returns whatever the healthy providers had. Returns the merged slice
// and the list of provider names that failed (in fetch order).
func aggregate(ctx context.Context) (models []Model, failed []string) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make(map[string][]Model, len(providers))
	)
	wg.Add(len(providers))
	for _, p := range providers {
		p := p
		go func() {
			defer wg.Done()
			ms, err := fetchProvider(ctx, p)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				slog.Warn("provider failed", "provider", p.Name, "err", err)
				failed = append(failed, p.Name)
				return
			}
			results[p.Name] = ms
		}()
	}
	wg.Wait()

	// Stitch slices back together in the original provider order.
	var ordered [][]Model
	for _, p := range providers {
		if s, ok := results[p.Name]; ok {
			ordered = append(ordered, s)
		}
	}
	return mergeModels(ordered), failed
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func handleModels(cache *modelCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		now := time.Now()
		if body, ok := cache.get(now); ok {
			writeModels(w, body, "HIT")
			return
		}

		// Bound the whole handler by the slowest upstream + a small margin
		// so a stuck provider can't hold the request open past its timeout.
		ctx, cancel := context.WithTimeout(r.Context(), upstreamTTL+time.Second)
		defer cancel()

		models, failed := aggregate(ctx)
		if len(models) == 0 {
			// Every provider failed — surface as 502 so the caller can
			// distinguish "discovery dead" from "discovery healthy, no models".
			slog.Error("all providers failed", "providers", failed)
			http.Error(w, "all providers failed", http.StatusBadGateway)
			return
		}

		payload, err := json.Marshal(outputResponse{Object: "list", Data: models})
		if err != nil {
			http.Error(w, "marshal failed", http.StatusInternalServerError)
			return
		}

		cache.set(payload, now.Add(cacheTTL))
		writeModels(w, payload, "MISS")
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func writeModels(w http.ResponseWriter, body []byte, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
	slog.Info("served models", "cache", status, "bytes", len(body))
}

func main() {
	// JSON to stdout for Promtail/Loki. Default-tag every line with the
	// service name so downstream filters can scope to this workload.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "agentgateway-discovery"))

	addr := ":" + envOr("PORT", "8080")
	cache := &modelCache{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", handleModels(cache))
	mux.HandleFunc("/healthz", handleHealthz)
	// Everything else falls through to a default 404 from net/http.

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", addr, "providers", len(providers))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("graceful shutdown error", "err", err)
	}
}