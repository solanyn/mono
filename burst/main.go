package main

import (
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctrl.SetLogger(zap.New())
	log := ctrl.Log.WithName("burst")

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	maxNodes := 4
	if v := os.Getenv("BURST_MAX_NODES"); v != "" {
		maxNodes, _ = strconv.Atoi(v)
	}
	idleTimeout := 10 * time.Minute
	if v := os.Getenv("BURST_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			idleTimeout = d
		}
	}
	namespace := "crossplane-system"
	if v := os.Getenv("BURST_NAMESPACE"); v != "" {
		namespace = v
	}

	if err := (&BurstReconciler{
		Client:      mgr.GetClient(),
		MaxNodes:    maxNodes,
		IdleTimeout: idleTimeout,
		Namespace:   namespace,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
