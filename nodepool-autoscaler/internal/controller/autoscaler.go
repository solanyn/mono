package controller

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/solanyn/mono/nodepool-autoscaler/api/v1alpha1"
	"github.com/solanyn/mono/nodepool-autoscaler/internal/metrics"
	"github.com/solanyn/mono/nodepool-autoscaler/internal/nodepool"
)

type scalerState struct {
	firstPendingSeen time.Time
	lastActivityTime time.Time
}

type Reconciler struct {
	client.Client
	Log *slog.Logger

	mu     sync.Mutex
	states map[types.NamespacedName]*scalerState
}

func (r *Reconciler) getState(key types.NamespacedName) *scalerState {
	if r.states == nil {
		r.states = make(map[types.NamespacedName]*scalerState)
	}
	if _, ok := r.states[key]; !ok {
		r.states[key] = &scalerState{}
	}
	return r.states[key]
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	scaler := &v1alpha1.NodePoolScaler{}
	if err := r.Get(ctx, req.NamespacedName, scaler); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	reconcileInterval := 30 * time.Second

	pendingPods, err := r.listPendingPods(ctx, scaler.Spec.Selector)
	if err != nil {
		r.Log.Error("listing pending pods", "error", err, "scaler", req.NamespacedName)
		return ctrl.Result{RequeueAfter: reconcileInterval}, nil
	}

	metrics.PendingPods.WithLabelValues(req.String()).Set(float64(len(pendingPods)))

	targetGVK := parseGVK(scaler.Spec.TargetRef)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(targetGVK)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      scaler.Spec.TargetRef.Name,
		Namespace: scaler.Spec.TargetRef.Namespace,
	}, obj); err != nil {
		r.Log.Error("getting target resource", "error", err, "scaler", req.NamespacedName)
		r.setCondition(ctx, scaler, "Ready", metav1.ConditionFalse, "TargetNotFound", fmt.Sprintf("target resource not found: %v", err))
		return ctrl.Result{RequeueAfter: reconcileInterval}, nil
	}

	sizeField := nodepool.ParseFieldPath(scaler.Spec.TargetRef.SizeField)
	maxSizeField := nodepool.ParseFieldPath(scaler.Spec.TargetRef.MaxSizeField)

	currentSize, _ := nodepool.GetFieldInt64(obj, sizeField)
	maxSize := scaler.Spec.Scaling.MaxSize
	if scaler.Spec.TargetRef.MaxSizeField != "" {
		if crMax, ok := nodepool.GetFieldInt64(obj, maxSizeField); ok && crMax < maxSize {
			maxSize = crMax
		}
	}

	metrics.TargetSize.WithLabelValues(req.String()).Set(float64(currentSize))

	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.getState(req.NamespacedName)

	if len(pendingPods) > 0 {
		now := time.Now()

		if state.firstPendingSeen.IsZero() {
			state.firstPendingSeen = now
		}

		debounce := scaler.Spec.Scaling.ScaleUpDebounce.Duration
		if now.Sub(state.firstPendingSeen) < debounce {
			return ctrl.Result{RequeueAfter: debounce - now.Sub(state.firstPendingSeen)}, nil
		}

		desired := calculateDesiredSize(pendingPods, scaler.Spec.Scaling.AllocatableCPUMillis)
		if desired > maxSize {
			desired = maxSize
		}
		if desired < scaler.Spec.Scaling.MinSize {
			desired = scaler.Spec.Scaling.MinSize
		}
		if desired < 1 {
			desired = 1
		}

		if desired > currentSize {
			r.Log.Info("scaling up", "current", currentSize, "desired", desired, "pending_pods", len(pendingPods), "scaler", req.NamespacedName)
			tc := nodepool.TargetConfig{
				Group:     targetGVK.Group,
				Version:   targetGVK.Version,
				Kind:      targetGVK.Kind,
				Name:      scaler.Spec.TargetRef.Name,
				Namespace: scaler.Spec.TargetRef.Namespace,
				SizeField: sizeField,
			}
			if err := nodepool.PatchTargetSize(ctx, r.Client, tc, desired); err != nil {
				r.Log.Error("patching target size", "error", err)
				return ctrl.Result{RequeueAfter: reconcileInterval}, nil
			}
			metrics.TargetSize.WithLabelValues(req.String()).Set(float64(desired))
			metrics.ScaleUpTotal.WithLabelValues(req.String()).Inc()
			metrics.LastScaleUpTime.WithLabelValues(req.String()).Set(float64(now.Unix()))

			nowMeta := metav1.NewTime(now)
			scaler.Status.LastScaleTime = &nowMeta
		}

		state.lastActivityTime = now
		metrics.IdleSeconds.WithLabelValues(req.String()).Set(0)

		scaler.Status.CurrentSize = currentSize
		scaler.Status.DesiredSize = desired
		scaler.Status.PendingPods = int64(len(pendingPods))
		scaler.Status.IdleSeconds = 0
		r.setCondition(ctx, scaler, "Ready", metav1.ConditionTrue, "ScalingUp", "pending pods detected")
	} else {
		state.firstPendingSeen = time.Time{}

		if state.lastActivityTime.IsZero() {
			state.lastActivityTime = time.Now()
		}

		idleDuration := time.Since(state.lastActivityTime)
		metrics.IdleSeconds.WithLabelValues(req.String()).Set(idleDuration.Seconds())

		idleTimeout := scaler.Spec.Scaling.IdleTimeout.Duration
		if currentSize > 0 && idleTimeout > 0 && idleDuration >= idleTimeout {
			r.Log.Info("scaling down to zero", "idle_duration", idleDuration, "scaler", req.NamespacedName)
			tc := nodepool.TargetConfig{
				Group:     targetGVK.Group,
				Version:   targetGVK.Version,
				Kind:      targetGVK.Kind,
				Name:      scaler.Spec.TargetRef.Name,
				Namespace: scaler.Spec.TargetRef.Namespace,
				SizeField: sizeField,
			}
			if err := nodepool.PatchTargetSize(ctx, r.Client, tc, 0); err != nil {
				r.Log.Error("patching target size to 0", "error", err)
				return ctrl.Result{RequeueAfter: reconcileInterval}, nil
			}
			metrics.TargetSize.WithLabelValues(req.String()).Set(0)
			metrics.ScaleDownTotal.WithLabelValues(req.String()).Inc()
			state.lastActivityTime = time.Time{}

			now := metav1.Now()
			scaler.Status.LastScaleTime = &now
		}

		scaler.Status.CurrentSize = currentSize
		scaler.Status.DesiredSize = 0
		scaler.Status.PendingPods = 0
		scaler.Status.IdleSeconds = int64(idleDuration.Seconds())
		r.setCondition(ctx, scaler, "Ready", metav1.ConditionTrue, "Idle", "no pending pods")
	}

	if err := r.Status().Update(ctx, scaler); err != nil {
		r.Log.Error("updating status", "error", err, "scaler", req.NamespacedName)
	}

	return ctrl.Result{RequeueAfter: reconcileInterval}, nil
}

func (r *Reconciler) setCondition(ctx context.Context, scaler *v1alpha1.NodePoolScaler, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i, c := range scaler.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				scaler.Status.Conditions[i].LastTransitionTime = now
			}
			scaler.Status.Conditions[i].Status = status
			scaler.Status.Conditions[i].Reason = reason
			scaler.Status.Conditions[i].Message = message
			scaler.Status.Conditions[i].ObservedGeneration = scaler.Generation
			return
		}
	}
	scaler.Status.Conditions = append(scaler.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: scaler.Generation,
	})
}

func calculateDesiredSize(pods []corev1.Pod, allocatableCPUMillis int64) int64 {
	if allocatableCPUMillis <= 0 {
		return int64(len(pods))
	}

	var totalCPUMillis int64
	for _, pod := range pods {
		var containerSum int64
		for _, c := range pod.Spec.Containers {
			if cpu := c.Resources.Requests.Cpu(); cpu != nil {
				containerSum += cpu.MilliValue()
			}
		}

		var initMax int64
		for _, c := range pod.Spec.InitContainers {
			if cpu := c.Resources.Requests.Cpu(); cpu != nil {
				if v := cpu.MilliValue(); v > initMax {
					initMax = v
				}
			}
		}

		podCPU := containerSum
		if initMax > podCPU {
			podCPU = initMax
		}
		totalCPUMillis += podCPU
	}

	if totalCPUMillis == 0 {
		return int64(len(pods))
	}

	return int64(math.Ceil(float64(totalCPUMillis) / float64(allocatableCPUMillis)))
}

func (r *Reconciler) listPendingPods(ctx context.Context, selector v1alpha1.PodSelector) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := r.List(ctx, &podList); err != nil {
		return nil, err
	}

	var result []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodPending {
			continue
		}
		if !matchesSelector(&pod, selector) {
			continue
		}
		if !isUnschedulable(&pod) {
			continue
		}
		result = append(result, pod)
	}
	return result, nil
}

func matchesSelector(pod *corev1.Pod, selector v1alpha1.PodSelector) bool {
	for k, v := range selector.NodeSelector {
		if pod.Spec.NodeSelector == nil {
			return false
		}
		if pod.Spec.NodeSelector[k] != v {
			return false
		}
	}

	for _, required := range selector.Tolerations {
		found := false
		for _, t := range pod.Spec.Tolerations {
			if t.Key == required.Key && t.Value == required.Value && string(t.Effect) == required.Effect {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func isUnschedulable(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == "Unschedulable" {
			return true
		}
	}
	return false
}

func parseGVK(ref v1alpha1.TargetRef) schema.GroupVersionKind {
	parts := strings.Split(ref.APIVersion, "/")
	if len(parts) == 2 {
		return schema.GroupVersionKind{Group: parts[0], Version: parts[1], Kind: ref.Kind}
	}
	return schema.GroupVersionKind{Version: ref.APIVersion, Kind: ref.Kind}
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodePoolScaler{}).
		Named("nodepool-autoscaler").
		Complete(r)
}
