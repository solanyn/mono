// Command openrouter-credits-exporter exposes OpenRouter account credit
// balance as Prometheus metrics.
//
// It polls GET /api/v1/credits on a fixed interval, caches the last successful
// sample and serves it on /metrics together with freshness and error
// indicators. It requires an OpenRouter *management* API key, supplied through
// the environment or a secret file; the key is never logged, never exported and
// never used as a metric label.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = log.New(os.Stderr, "", log.LstdFlags|log.LUTC)

// Runner owns the poll loop.
type Runner struct {
	cfg       Config
	client    *Client
	collector *Collector
	logger    *log.Logger
}

func NewRunner(cfg Config, collector *Collector, lg *log.Logger) *Runner {
	return &Runner{
		cfg:       cfg,
		client:    NewClient(cfg.APIURL, cfg.ManagementKey, cfg.Timeout),
		collector: collector,
		logger:    lg,
	}
}

// pollOnce performs exactly one bounded attempt and records the outcome. There
// are no retries; the next tick is the retry.
func (r *Runner) pollOnce(ctx context.Context) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	credits, err := r.client.FetchCredits(ctx)
	duration := time.Since(started)

	if err != nil {
		category := CategoryOf(err)
		r.collector.RecordFailure(category, started, duration)
		r.logger.Printf("level=error msg=\"openrouter credits poll failed\" category=%s status=%s duration=%s",
			category, r.collector.redacted(err.Error()), duration.Round(time.Millisecond))
		return
	}

	r.collector.RecordSuccess(credits, started, duration)
	r.logger.Printf("level=info msg=\"openrouter credits poll ok\" total_credits_usd=%.4f total_usage_usd=%.4f remaining_usd=%.4f duration=%s",
		credits.TotalCredits, credits.TotalUsage, credits.Remaining(), duration.Round(time.Millisecond))
}

// Run polls until ctx is cancelled. Ticks are synchronous, so runs can never
// overlap even if the API is slow.
func (r *Runner) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.pollOnce(ctx)
		}
	}
}

func (r *Runner) routes(reg *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", r.healthHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("openrouter-credits-exporter\n\n/metrics\n/health\n"))
	})
	return mux
}

func (r *Runner) healthHandler(w http.ResponseWriter, req *http.Request) {
	status, ok := r.collector.Health(r.cfg.Interval)
	code := http.StatusOK
	if !ok {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(status)
}

func main() {
	if err := run(); err != nil {
		logger.Printf("level=error msg=\"exporter exited\" error=%q", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		if errors.Is(err, ErrMissingManagementKey) {
			return errors.New("missing management key: set OPENROUTER_MANAGEMENT_KEY_FILE (secret file) or OPENROUTER_MANAGEMENT_KEY")
		}
		return err
	}

	logger.Printf("level=info msg=\"starting openrouter credits exporter\" addr=%s interval=%s timeout=%s stale_after=%s key_source=%s",
		cfg.Addr, cfg.Interval, cfg.Timeout, cfg.StaleAfter, cfg.KeySource)

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	collector := NewCollector(reg, cfg.StaleAfter, cfg.ManagementKey)
	runner := NewRunner(cfg, collector, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Prime the cache so /health is meaningful immediately and a misconfigured
	// key is visible at startup rather than after one interval.
	runner.pollOnce(ctx)
	go runner.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           runner.routes(reg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Printf("level=info msg=\"shutting down\"")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
