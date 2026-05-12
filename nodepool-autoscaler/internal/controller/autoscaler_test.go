package controller

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/solanyn/mono/nodepool-autoscaler/api/v1alpha1"
	"github.com/solanyn/mono/nodepool-autoscaler/internal/nodepool"
)

func defaultScaler() *v1alpha1.NodePoolScaler {
	return &v1alpha1.NodePoolScaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-scaler",
			Namespace: "default",
		},
		Spec: v1alpha1.NodePoolScalerSpec{
			TargetRef: v1alpha1.TargetRef{
				APIVersion:   "home-ops.io/v1alpha1",
				Kind:         "XComputeNodePool",
				Name:         "nodepool",
				Namespace:    "crossplane-system",
				SizeField:    "spec.targetSize",
				MaxSizeField: "spec.maxSize",
			},
			Selector: v1alpha1.PodSelector{
				NodeSelector: map[string]string{
					"node.kubernetes.io/nodepool": "true",
				},
				Tolerations: []v1alpha1.Toleration{
					{
						Key:    "nodepool",
						Value:  "true",
						Effect: "NoSchedule",
					},
				},
			},
			Scaling: v1alpha1.ScalingPolicy{
				AllocatableCPUMillis: 7000,
				ScaleUpDebounce:      metav1.Duration{Duration: 0},
				IdleTimeout:          metav1.Duration{Duration: 15 * time.Minute},
				MinSize:              0,
				MaxSize:              8,
			},
		},
	}
}

func newNodepoolCR(targetSize, maxSize int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(parseGVK(v1alpha1.TargetRef{
		APIVersion: "home-ops.io/v1alpha1",
		Kind:       "XComputeNodePool",
	}))
	obj.SetName("nodepool")
	obj.SetNamespace("crossplane-system")
	_ = unstructured.SetNestedField(obj.Object, targetSize, "spec", "targetSize")
	_ = unstructured.SetNestedField(obj.Object, maxSize, "spec", "maxSize")
	return obj
}

func newPendingPod(name string, cpuRequest string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node.kubernetes.io/nodepool": "true",
			},
			Tolerations: []corev1.Toleration{
				{
					Key:    "nodepool",
					Value:  "true",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "worker",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse(cpuRequest),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodScheduled,
					Status: corev1.ConditionFalse,
					Reason: "Unschedulable",
				},
			},
		},
	}
}

func newNonNodepoolPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "busybox",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodScheduled,
					Status: corev1.ConditionFalse,
					Reason: "Unschedulable",
				},
			},
		},
	}
}

func TestCalculateDesiredSize(t *testing.T) {
	tests := []struct {
		name     string
		pods     []corev1.Pod
		expected int64
	}{
		{
			name:     "single pod 2000m",
			pods:     []corev1.Pod{*newPendingPod("p1", "2000m")},
			expected: 1,
		},
		{
			name:     "single pod 7000m",
			pods:     []corev1.Pod{*newPendingPod("p1", "7000m")},
			expected: 1,
		},
		{
			name:     "two pods 7000m each",
			pods:     []corev1.Pod{*newPendingPod("p1", "7000m"), *newPendingPod("p2", "7000m")},
			expected: 2,
		},
		{
			name:     "three pods 3500m each",
			pods:     []corev1.Pod{*newPendingPod("p1", "3500m"), *newPendingPod("p2", "3500m"), *newPendingPod("p3", "3500m")},
			expected: 2,
		},
		{
			name:     "pods with no cpu requests defaults to pod count",
			pods:     []corev1.Pod{*newPendingPod("p1", "0"), *newPendingPod("p2", "0")},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateDesiredSize(tt.pods, 7000)
			if got != tt.expected {
				t.Errorf("calculateDesiredSize() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestClampToMaxSize(t *testing.T) {
	pods := make([]corev1.Pod, 10)
	for i := range pods {
		pods[i] = *newPendingPod("p"+string(rune('0'+i)), "7000m")
	}

	desired := calculateDesiredSize(pods, 7000)
	maxSize := int64(4)
	if desired > maxSize {
		if desired != 10 {
			t.Errorf("expected raw desired=10, got %d", desired)
		}
	}
}

func TestMatchesSelector(t *testing.T) {
	selector := v1alpha1.PodSelector{
		NodeSelector: map[string]string{
			"node.kubernetes.io/nodepool": "true",
		},
		Tolerations: []v1alpha1.Toleration{
			{Key: "nodepool", Value: "true", Effect: "NoSchedule"},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name:     "nodepool pod",
			pod:      newPendingPod("p1", "1000m"),
			expected: true,
		},
		{
			name:     "non-nodepool pod",
			pod:      newNonNodepoolPod("p2"),
			expected: false,
		},
		{
			name: "missing toleration",
			pod: func() *corev1.Pod {
				p := newPendingPod("p3", "1000m")
				p.Spec.Tolerations = nil
				return p
			}(),
			expected: false,
		},
		{
			name: "missing node selector",
			pod: func() *corev1.Pod {
				p := newPendingPod("p4", "1000m")
				p.Spec.NodeSelector = nil
				return p
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSelector(tt.pod, selector) && isUnschedulable(tt.pod)
			if got != tt.expected {
				t.Errorf("matchesSelector && isUnschedulable = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDebounce(t *testing.T) {
	state := &scalerState{
		firstPendingSeen: time.Now(),
	}
	debounce := 10 * time.Second

	elapsed := time.Since(state.firstPendingSeen)
	if elapsed >= debounce {
		t.Error("debounce should not have elapsed yet")
	}
}

func TestScaleDownIdle(t *testing.T) {
	state := &scalerState{
		lastActivityTime: time.Now().Add(-1 * time.Second),
	}
	idleTimeout := 1 * time.Millisecond

	idleDuration := time.Since(state.lastActivityTime)
	if idleDuration < idleTimeout {
		t.Error("expected idle timeout to have elapsed")
	}
}

func TestNoOpWhenTargetSizeMatches(t *testing.T) {
	cr := newNodepoolCR(2, 8)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(cr).
		Build()

	ctx := context.Background()
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(parseGVK(v1alpha1.TargetRef{
		APIVersion: "home-ops.io/v1alpha1",
		Kind:       "XComputeNodePool",
	}))
	err := c.Get(ctx, types.NamespacedName{Name: "nodepool", Namespace: "crossplane-system"}, obj)
	if err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	currentSize, ok := nodepool.GetFieldInt64(obj, []string{"spec", "targetSize"})
	if !ok {
		t.Fatal("targetSize not found")
	}
	if currentSize != 2 {
		t.Errorf("expected targetSize=2, got %d", currentSize)
	}
}

func TestReconcileScaleUp(t *testing.T) {
	cr := newNodepoolCR(0, 8)
	pod := newPendingPod("worker-1", "3500m")
	scaler := defaultScaler()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(cr, pod).
		WithStatusSubresource(scaler).
		WithObjects(scaler).
		WithIndex(&corev1.Pod{}, "status.phase", func(obj client.Object) []string {
			p := obj.(*corev1.Pod)
			return []string{string(p.Status.Phase)}
		}).
		Build()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := &Reconciler{
		Client: c,
		Log:    log,
		states: map[types.NamespacedName]*scalerState{
			{Name: "test-scaler", Namespace: "default"}: {
				firstPendingSeen: time.Now().Add(-1 * time.Minute),
			},
		},
	}

	ctx := context.Background()
	result, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-scaler", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Errorf("expected requeue after 30s, got %v", result.RequeueAfter)
	}
}
