package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestCollector(t *testing.T, staleAfter time.Duration) (*Collector, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	return NewCollector(reg, staleAfter, testKey), reg
}

// gatherText renders the registry in Prometheus exposition format.
func gatherText(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var sb strings.Builder
	for _, fam := range families {
		for _, m := range fam.Metric {
			name := fam.GetName()
			if len(m.Label) == 0 {
				sb.WriteString(name + " " + formatValue(m) + "\n")
				continue
			}
			labels := make([]string, 0, len(m.Label))
			for _, l := range m.Label {
				labels = append(labels, l.GetName()+`="`+l.GetValue()+`"`)
			}
			sb.WriteString(name + "{" + strings.Join(labels, ",") + "} " + formatValue(m) + "\n")
		}
	}
	return sb.String()
}

func formatValue(m *dto.Metric) string {
	switch {
	case m.Gauge != nil:
		return strconv.FormatFloat(m.Gauge.GetValue(), 'g', -1, 64)
	case m.Counter != nil:
		return strconv.FormatFloat(m.Counter.GetValue(), 'g', -1, 64)
	default:
		return "0"
	}
}

func TestCollectorExportsCachedSample(t *testing.T) {
	c, reg := newTestCollector(t, 15*time.Minute)
	started := time.Now()
	c.RecordSuccess(Credits{TotalCredits: 100.5, TotalUsage: 37.25}, started, 120*time.Millisecond)

	text := gatherText(t, reg)
	for _, want := range []string{
		`openrouter_credits_total_usd 100.5`,
		`openrouter_credits_used_usd 37.25`,
		`openrouter_credits_remaining_usd 63.25`,
		`openrouter_exporter_up 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "openrouter_exporter_last_success_timestamp_seconds") {
		t.Fatalf("last success timestamp missing:\n%s", text)
	}
	if !strings.Contains(text, "openrouter_exporter_sample_age_seconds") {
		t.Fatalf("sample age missing:\n%s", text)
	}
	if strings.Contains(text, testKey) {
		t.Fatal("management key appeared in metrics output")
	}
}

func TestCollectorKeepsLastGoodSampleAfterFailure(t *testing.T) {
	c, reg := newTestCollector(t, 15*time.Minute)
	started := time.Now()
	c.RecordSuccess(Credits{TotalCredits: 20, TotalUsage: 5}, started, time.Millisecond)
	c.RecordFailure(CategoryAuth, started.Add(time.Minute), 2*time.Millisecond)

	text := gatherText(t, reg)
	// The cached balance must survive a failed poll; freshness carries the fault.
	if !strings.Contains(text, `openrouter_credits_remaining_usd 15`) {
		t.Fatalf("cached sample was dropped after a failure:\n%s", text)
	}
	if !strings.Contains(text, `openrouter_exporter_up 0`) {
		t.Fatalf("up was not cleared after a failure:\n%s", text)
	}
	if !strings.Contains(text, `openrouter_exporter_errors_total{category="auth"} 1`) {
		t.Fatalf("auth error counter not incremented:\n%s", text)
	}
	if !strings.Contains(text, `openrouter_exporter_polls_total{outcome="failure"} 1`) {
		t.Fatalf("failure poll counter not incremented:\n%s", text)
	}
	if _, ok := c.Health(time.Minute); ok {
		t.Fatal("health reported healthy while the last poll was failing")
	}
}

func TestCollectorExportsNoBalanceBeforeFirstSuccess(t *testing.T) {
	_, reg := newTestCollector(t, time.Minute)
	text := gatherText(t, reg)
	if strings.Contains(text, "openrouter_credits_remaining_usd") {
		t.Fatalf("balance exported before any successful poll:\n%s", text)
	}
	if !strings.Contains(text, `openrouter_exporter_up 0`) {
		t.Fatalf("up not 0 before first poll:\n%s", text)
	}
}

func TestCollectorPreInitialisesErrorCategories(t *testing.T) {
	_, reg := newTestCollector(t, time.Minute)
	text := gatherText(t, reg)
	for _, cat := range Categories {
		want := `openrouter_exporter_errors_total{category="` + string(cat) + `"} 0`
		if !strings.Contains(text, want) {
			t.Fatalf("expected stable zero series %q in:\n%s", want, text)
		}
	}
}

func TestHealthStates(t *testing.T) {
	c, _ := newTestCollector(t, 50*time.Millisecond)

	status, ok := c.Health(time.Minute)
	if ok || status.Status != "never_succeeded" {
		t.Fatalf("expected never_succeeded, got %+v ok=%v", status, ok)
	}

	c.RecordSuccess(Credits{TotalCredits: 1, TotalUsage: 0}, time.Now(), time.Millisecond)
	status, ok = c.Health(time.Minute)
	if !ok || status.Status != "ok" {
		t.Fatalf("expected ok, got %+v ok=%v", status, ok)
	}
	if status.SampleAgeSeconds == nil || status.LastSuccess == nil {
		t.Fatalf("health should report sample age and last success: %+v", status)
	}

	time.Sleep(80 * time.Millisecond)
	status, ok = c.Health(time.Minute)
	if ok || status.Status != "stale" {
		t.Fatalf("expected stale, got %+v ok=%v", status, ok)
	}
}

func TestHealthEndpoint(t *testing.T) {
	c, reg := newTestCollector(t, time.Minute)
	cfg := Config{Interval: time.Minute}
	runner := NewRunner(cfg, c, testLogger())
	srv := httptest.NewServer(runner.routes(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 before a successful poll", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var status HealthStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("health body is not JSON: %v (%s)", err, body)
	}
	if status.Status != "never_succeeded" {
		t.Fatalf("unexpected health status %q", status.Status)
	}
	if strings.Contains(string(body), testKey) || strings.Contains(string(body), "Bearer") {
		t.Fatalf("health response leaked credential material: %s", body)
	}

	c.RecordSuccess(Credits{TotalCredits: 10, TotalUsage: 4}, time.Now(), time.Millisecond)
	resp2, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 after a successful poll", resp2.StatusCode)
	}
}

func TestMetricsEndpointServesExposition(t *testing.T) {
	c, reg := newTestCollector(t, time.Minute)
	c.RecordSuccess(Credits{TotalCredits: 5, TotalUsage: 1}, time.Now(), time.Millisecond)
	runner := NewRunner(Config{Interval: time.Minute}, c, testLogger())
	srv := httptest.NewServer(runner.routes(reg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(text, "openrouter_credits_remaining_usd 4") {
		t.Fatalf("metrics endpoint missing the balance:\n%s", text)
	}
	if strings.Contains(text, testKey) {
		t.Fatal("management key appeared on /metrics")
	}
}
