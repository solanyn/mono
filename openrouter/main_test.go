package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func testLogger() *log.Logger { return log.New(&strings.Builder{}, "", 0) }

// countingServer serves a queued sequence of responses and counts requests.
func recordingServer(t *testing.T, bodies []string, statuses []int) (*httptest.Server, *int) {
	t.Helper()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := calls
		calls++
		if i < len(statuses) && statuses[i] != 0 {
			w.WriteHeader(statuses[i])
		}
		if i < len(bodies) {
			_, _ = w.Write([]byte(bodies[i]))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestPollOnceRecordsSuccess(t *testing.T) {
	srv, calls := recordingServer(t,
		[]string{`{"data":{"total_credits":50,"total_usage":20}}`}, []int{0})

	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	cfg := Config{APIURL: srv.URL, ManagementKey: testKey, Timeout: 2 * time.Second, Interval: time.Minute}
	runner := NewRunner(cfg, collector, testLogger())

	runner.pollOnce(context.Background())
	if *calls != 1 {
		t.Fatalf("expected exactly one attempt, got %d", *calls)
	}
	status, ok := collector.Health(time.Minute)
	if !ok || status.Status != "ok" {
		t.Fatalf("expected healthy status after a good poll, got %+v ok=%v", status, ok)
	}

	text := gatherText(t, reg)
	if !strings.Contains(text, `openrouter_credits_remaining_usd 30`) {
		t.Fatalf("remaining balance not exported:\n%s", text)
	}
}

func TestPollOnceRecordsFailureWithoutRetrying(t *testing.T) {
	srv, calls := recordingServer(t, []string{`nope`}, []int{http.StatusUnauthorized})

	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	cfg := Config{APIURL: srv.URL, ManagementKey: testKey, Timeout: 2 * time.Second, Interval: time.Minute}
	runner := NewRunner(cfg, collector, testLogger())

	runner.pollOnce(context.Background())
	if *calls != 1 {
		t.Fatalf("expected exactly one attempt with no retries, got %d", *calls)
	}
	status, ok := collector.Health(time.Minute)
	if ok || status.Status != "never_succeeded" {
		t.Fatalf("expected never_succeeded, got %+v ok=%v", status, ok)
	}

	text := gatherText(t, reg)
	if !strings.Contains(text, `openrouter_exporter_errors_total{category="auth"} 1`) {
		t.Fatalf("auth failure not recorded:\n%s", text)
	}
	if strings.Contains(text, "openrouter_credits_remaining_usd") {
		t.Fatalf("balance exported without a successful poll:\n%s", text)
	}
}

func TestPollOnceTimeoutIsBounded(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })

	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	cfg := Config{APIURL: srv.URL, ManagementKey: testKey, Timeout: 150 * time.Millisecond, Interval: time.Minute}
	runner := NewRunner(cfg, collector, testLogger())

	start := time.Now()
	runner.pollOnce(context.Background())
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("poll was not bounded by the timeout, elapsed %s", elapsed)
	}
	text := gatherText(t, reg)
	if !strings.Contains(text, `openrouter_exporter_errors_total{category="network"} 1`) {
		t.Fatalf("timeout not categorised as network:\n%s", text)
	}
}

func TestRunnerStopsOnContextCancel(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	cfg := Config{APIURL: "http://127.0.0.1:1", ManagementKey: testKey, Timeout: 100 * time.Millisecond, Interval: 5 * time.Millisecond}
	runner := NewRunner(cfg, collector, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runner.Run(ctx); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestLogsNeverContainTheManagementKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key ` + testKey + `"}`))
	}))
	defer srv.Close()

	var buf strings.Builder
	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	cfg := Config{APIURL: srv.URL, ManagementKey: testKey, Timeout: 2 * time.Second, Interval: time.Minute}
	runner := NewRunner(cfg, collector, log.New(&buf, "", 0))
	runner.pollOnce(context.Background())

	logged := buf.String()
	if logged == "" {
		t.Fatal("expected a failure log line")
	}
	if strings.Contains(logged, testKey) {
		t.Fatalf("management key written to logs: %s", logged)
	}
	if strings.Contains(logged, "bad key") {
		t.Fatalf("upstream response body written to logs: %s", logged)
	}
	if !strings.Contains(logged, "category=auth") {
		t.Fatalf("failure log missing the bounded category: %s", logged)
	}
}

func TestRoutesRootAndNotFound(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewCollector(reg, time.Minute, testKey)
	runner := NewRunner(Config{Interval: time.Minute}, collector, testLogger())
	srv := httptest.NewServer(runner.routes(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("root status = %d", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", resp2.StatusCode)
	}
}
