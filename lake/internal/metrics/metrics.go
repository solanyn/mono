package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	IngestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lake_ingest_total",
		Help: "Total number of successful ingests",
	}, []string{"source"})

	IngestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lake_ingest_duration_seconds",
		Help:    "Duration of ingest operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"source"})

	IngestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lake_ingest_errors_total",
		Help: "Total number of ingest errors",
	}, []string{"source"})

	PromoteTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lake_promote_total",
		Help: "Total number of successful promotions",
	}, []string{"layer"})

	LastIngestTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lake_last_ingest_timestamp",
		Help: "Unix timestamp of last successful ingest",
	}, []string{"source"})
)
