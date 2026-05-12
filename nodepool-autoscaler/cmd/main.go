package main

import (
	"log/slog"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "github.com/solanyn/mono/nodepool-autoscaler/api/v1alpha1"
	"github.com/solanyn/mono/nodepool-autoscaler/internal/controller"
	_ "github.com/solanyn/mono/nodepool-autoscaler/internal/metrics"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         true,
		LeaderElectionID:       "nodepool-autoscaler.home-ops.io",
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}:              {},
				&v1alpha1.NodePoolScaler{}: {},
			},
		},
	})
	if err != nil {
		log.Error("unable to create manager", "error", err)
		os.Exit(1)
	}

	reconciler := &controller.Reconciler{
		Client: mgr.GetClient(),
		Log:    log,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Error("unable to setup controller", "error", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error("manager exited with error", "error", err)
		os.Exit(1)
	}
}
