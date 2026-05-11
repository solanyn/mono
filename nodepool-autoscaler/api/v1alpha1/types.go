package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TargetRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	SizeField  string `json:"sizeField"`
	MaxSizeField string `json:"maxSizeField,omitempty"`
}

type ScalingPolicy struct {
	AllocatableCPUMillis int64            `json:"allocatableCPUMillis"`
	ScaleUpDebounce      metav1.Duration  `json:"scaleUpDebounce,omitempty"`
	IdleTimeout          metav1.Duration  `json:"idleTimeout,omitempty"`
	MinSize              int64            `json:"minSize,omitempty"`
	MaxSize              int64            `json:"maxSize"`
}

type PodSelector struct {
	NodeSelector map[string]string `json:"nodeSelector"`
	Tolerations  []Toleration      `json:"tolerations,omitempty"`
}

type Toleration struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect,omitempty"`
}

type NodePoolScalerSpec struct {
	TargetRef TargetRef     `json:"targetRef"`
	Selector  PodSelector   `json:"selector"`
	Scaling   ScalingPolicy `json:"scaling"`
}

type NodePoolScalerStatus struct {
	CurrentSize    int64        `json:"currentSize,omitempty"`
	DesiredSize    int64        `json:"desiredSize,omitempty"`
	PendingPods    int64        `json:"pendingPods,omitempty"`
	LastScaleTime  *metav1.Time `json:"lastScaleTime,omitempty"`
	IdleSeconds    float64      `json:"idleSeconds,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type NodePoolScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolScalerSpec   `json:"spec,omitempty"`
	Status NodePoolScalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NodePoolScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePoolScaler `json:"items"`
}
