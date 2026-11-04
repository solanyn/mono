package main

import (
	"math"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type snapshot struct {
	haveSample   bool
	credits      Credits
	up           bool
	lastAttempt  time.Time
	lastSuccess  time.Time
	lastDuration time.Duration
	lastCategory Category
}

// Collector caches the most recent successful sample and the poll outcome, and
// exposes both as Prometheus metrics. Failed polls never clear the last good
// sample: freshness is signalled separately, so a scrape cannot silently read a
// zero balance.
type Collector struct {
	mu         sync.RWMutex
	snap       snapshot
	staleAfter time.Duration
	secret     string

	polls  *prometheus.CounterVec
	errors *prometheus.CounterVec
	descs  collectorDescs
}

type collectorDescs struct {
	totalCredits *prometheus.Desc
	totalUsage   *prometheus.Desc
	remaining    *prometheus.Desc
	up           *prometheus.Desc
	lastSuccess  *prometheus.Desc
	lastAttempt  *prometheus.Desc
	pollDuration *prometheus.Desc
	sampleAge    *prometheus.Desc
}

// NewCollector registers the collector and its counters with reg. secret is the
// management key; it is only ever used to redact output, never as a metric
// value or label.
func NewCollector(reg prometheus.Registerer, staleAfter time.Duration, secret string) *Collector {
	ns := "openrouter"
	sub := "exporter"
	c := &Collector{
		staleAfter: staleAfter,
		secret:     secret,
		descs: collectorDescs{
			totalCredits: prometheus.NewDesc(ns+"_credits_total_usd",
				"Total credits purchased on the OpenRouter account, in USD.", nil, nil),
			totalUsage: prometheus.NewDesc(ns+"_credits_used_usd",
				"Total credits consumed on the OpenRouter account, in USD.", nil, nil),
			remaining: prometheus.NewDesc(ns+"_credits_remaining_usd",
				"Remaining OpenRouter account credit in USD (total credits minus total usage).", nil, nil),
			up: prometheus.NewDesc(ns+"_"+sub+"_up",
				"Whether the cached OpenRouter sample is usable: 1 when the last poll succeeded, 0 otherwise.", nil, nil),
			lastSuccess: prometheus.NewDesc(ns+"_"+sub+"_last_success_timestamp_seconds",
				"Unix timestamp of the last successful OpenRouter credits poll; 0 when no poll has ever succeeded.", nil, nil),
			lastAttempt: prometheus.NewDesc(ns+"_"+sub+"_last_attempt_timestamp_seconds",
				"Unix timestamp of the most recent OpenRouter credits poll attempt; 0 when no poll has run yet.", nil, nil),
			pollDuration: prometheus.NewDesc(ns+"_"+sub+"_poll_duration_seconds",
				"Duration of the most recent OpenRouter credits poll attempt, in seconds.", nil, nil),
			sampleAge: prometheus.NewDesc(ns+"_"+sub+"_sample_age_seconds",
				"Age of the cached OpenRouter credits sample, in seconds. Only exported once a poll has succeeded.", nil, nil),
		},
		polls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "polls_total",
			Help:      "Total number of OpenRouter credits poll attempts by outcome.",
		}, []string{"outcome"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "errors_total",
			Help:      "Total number of failed OpenRouter credits polls by bounded failure category.",
		}, []string{"category"}),
	}

	// Stable zero-valued series so alerts and dashboards do not flap on first use.
	c.polls.WithLabelValues("success").Add(0)
	c.polls.WithLabelValues("failure").Add(0)
	for _, cat := range Categories {
		c.errors.WithLabelValues(string(cat)).Add(0)
	}

	if reg != nil {
		reg.MustRegister(c)
		reg.MustRegister(c.polls, c.errors)
	}
	return c
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descs.totalCredits
	ch <- c.descs.totalUsage
	ch <- c.descs.remaining
	ch <- c.descs.up
	ch <- c.descs.lastSuccess
	ch <- c.descs.lastAttempt
	ch <- c.descs.pollDuration
	ch <- c.descs.sampleAge
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	s := c.snap
	c.mu.RUnlock()

	now := time.Now()
	ch <- prometheus.MustNewConstMetric(c.descs.up, prometheus.GaugeValue, boolToFloat(s.up))
	ch <- prometheus.MustNewConstMetric(c.descs.lastSuccess, prometheus.GaugeValue, unixOrZero(s.lastSuccess))
	ch <- prometheus.MustNewConstMetric(c.descs.lastAttempt, prometheus.GaugeValue, unixOrZero(s.lastAttempt))
	ch <- prometheus.MustNewConstMetric(c.descs.pollDuration, prometheus.GaugeValue, s.lastDuration.Seconds())

	if s.haveSample && !s.lastSuccess.IsZero() {
		ch <- prometheus.MustNewConstMetric(c.descs.totalCredits, prometheus.GaugeValue, s.credits.TotalCredits)
		ch <- prometheus.MustNewConstMetric(c.descs.totalUsage, prometheus.GaugeValue, s.credits.TotalUsage)
		ch <- prometheus.MustNewConstMetric(c.descs.remaining, prometheus.GaugeValue, s.credits.Remaining())
		ch <- prometheus.MustNewConstMetric(c.descs.sampleAge, prometheus.GaugeValue, now.Sub(s.lastSuccess).Seconds())
	}
}

// RecordSuccess caches a good sample.
func (c *Collector) RecordSuccess(credits Credits, started time.Time, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.haveSample = true
	c.snap.credits = credits
	c.snap.up = true
	c.snap.lastAttempt = started
	c.snap.lastSuccess = time.Now()
	c.snap.lastDuration = duration
	c.snap.lastCategory = ""
	c.polls.WithLabelValues("success").Inc()
}

// RecordFailure records a bad poll while retaining the last good sample.
func (c *Collector) RecordFailure(category Category, started time.Time, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.up = false
	c.snap.lastAttempt = started
	c.snap.lastDuration = duration
	c.snap.lastCategory = category
	c.polls.WithLabelValues("failure").Inc()
	c.errors.WithLabelValues(string(category)).Inc()
}

// HealthStatus is a JSON-safe view of exporter health. It carries no secrets.
type HealthStatus struct {
	Status            string   `json:"status"`
	Up                bool     `json:"up"`
	ErrorCategory     Category `json:"error_category,omitempty"`
	SampleAgeSeconds  *float64 `json:"sample_age_seconds,omitempty"`
	LastSuccess       *string  `json:"last_success,omitempty"`
	PollIntervalSecs  float64  `json:"poll_interval_seconds"`
	StaleAfterSeconds float64  `json:"stale_after_seconds"`
}

// Health reports whether the cached sample is fresh enough to trust. ok is
// false when the exporter has never succeeded, the last poll failed, or the
// cached sample is older than staleAfter.
func (c *Collector) Health(interval time.Duration) (HealthStatus, bool) {
	c.mu.RLock()
	s := c.snap
	c.mu.RUnlock()

	status := HealthStatus{
		PollIntervalSecs:  interval.Seconds(),
		StaleAfterSeconds: c.staleAfter.Seconds(),
		Up:                s.up,
		ErrorCategory:     s.lastCategory,
	}
	if s.haveSample {
		age := time.Since(s.lastSuccess).Seconds()
		status.SampleAgeSeconds = &age
		ts := s.lastSuccess.UTC().Format(time.RFC3339)
		status.LastSuccess = &ts
	}

	switch {
	case !s.haveSample:
		status.Status = "never_succeeded"
		return status, false
	case !s.up:
		status.Status = "failing"
		return status, false
	case time.Since(s.lastSuccess) > c.staleAfter:
		status.Status = "stale"
		return status, false
	default:
		status.Status = "ok"
		return status, true
	}
}

func (c *Collector) redacted(s string) string { return redact(s, c.secret) }

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func unixOrZero(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.Unix())
}

func isNonFinite(f float64) bool {
	return math.IsNaN(f) || math.IsInf(f, 0)
}
