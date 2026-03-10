package metrics

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Metrics struct {
	CacheHits      atomic.Int64
	CacheMisses    atomic.Int64
	ColdHits       atomic.Int64
	DomainRequests atomic.Int64
	QuotaUsed      atomic.Int64
	QueueDepth     atomic.Int64
	QueueDrained   atomic.Int64
	SalesIngested  atomic.Int64
}

var Global = &Metrics{}

func (m *Metrics) RecordCacheHit()   { m.CacheHits.Add(1) }
func (m *Metrics) RecordCacheMiss()  { m.CacheMisses.Add(1) }
func (m *Metrics) RecordColdHit()    { m.ColdHits.Add(1) }
func (m *Metrics) RecordDomainReq()  { m.DomainRequests.Add(1) }
func (m *Metrics) RecordQueueDrain() { m.QueueDrained.Add(1) }

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := Global
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		writeMetric(w, "yield_cache_hits_total", m.CacheHits.Load())
		writeMetric(w, "yield_cache_misses_total", m.CacheMisses.Load())
		writeMetric(w, "yield_cold_hits_total", m.ColdHits.Load())
		writeMetric(w, "yield_domain_requests_total", m.DomainRequests.Load())
		writeMetric(w, "yield_quota_used", m.QuotaUsed.Load())
		writeMetric(w, "yield_queue_depth", m.QueueDepth.Load())
		writeMetric(w, "yield_queue_drained_total", m.QueueDrained.Load())
		writeMetric(w, "yield_sales_ingested_total", m.SalesIngested.Load())
		writeMetric(w, "yield_uptime_seconds", int64(time.Since(startTime).Seconds()))
	}
}

var startTime = time.Now()

func writeMetric(w http.ResponseWriter, name string, value int64) {
	w.Write([]byte(name + " " + strconv.FormatInt(value, 10) + "\n"))
}
