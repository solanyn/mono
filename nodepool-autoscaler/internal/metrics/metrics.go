package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	TargetSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_target_size",
		Help: "Current targetSize the controller has set",
	}, []string{"scaler"})

	PendingPods = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_pending_pods",
		Help: "Number of pending nodepool pods",
	}, []string{"scaler"})

	ScaleUpTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nodepool_autoscaler_scale_up_total",
		Help: "Number of scale-up events",
	}, []string{"scaler"})

	ScaleDownTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nodepool_autoscaler_scale_down_total",
		Help: "Number of scale-down events",
	}, []string{"scaler"})

	LastScaleUpTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_last_scale_up_time",
		Help: "Unix timestamp of last scale-up",
	}, []string{"scaler"})

	IdleSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodepool_autoscaler_idle_seconds",
		Help: "Seconds since last pod activity",
	}, []string{"scaler"})
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
