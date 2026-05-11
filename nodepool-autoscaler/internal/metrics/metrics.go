package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	TargetSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_target_size",
		Help: "Current targetSize the controller has set",
	})

	PendingPods = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_pending_pods",
		Help: "Number of pending nodepool pods",
	})

	ScaleUpTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "nodepool_autoscaler_scale_up_total",
		Help: "Number of scale-up events",
	})

	ScaleDownTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "nodepool_autoscaler_scale_down_total",
		Help: "Number of scale-down events",
	})

	LastScaleUpTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_last_scale_up_time",
		Help: "Unix timestamp of last scale-up",
	})

	IdleSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_idle_seconds",
		Help: "Seconds since last pod activity",
	})
)

func init() {
	metrics.Registry.MustRegister(
		TargetSize,
		PendingPods,
		ScaleUpTotal,
		ScaleDownTotal,
		LastScaleUpTime,
		IdleSeconds,
	)
}
