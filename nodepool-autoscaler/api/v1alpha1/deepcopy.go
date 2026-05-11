package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *NodePoolScaler) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *NodePoolScaler) DeepCopy() *NodePoolScaler {
	if in == nil {
		return nil
	}
	out := new(NodePoolScaler)
	in.DeepCopyInto(out)
	return out
}

func (in *NodePoolScaler) DeepCopyInto(out *NodePoolScaler) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *NodePoolScalerSpec) DeepCopyInto(out *NodePoolScalerSpec) {
	*out = *in
	out.TargetRef = in.TargetRef
	in.Selector.DeepCopyInto(&out.Selector)
	out.Scaling = in.Scaling
}

func (in *PodSelector) DeepCopyInto(out *PodSelector) {
	*out = *in
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string, len(in.NodeSelector))
		for k, v := range in.NodeSelector {
			out.NodeSelector[k] = v
		}
	}
	if in.Tolerations != nil {
		out.Tolerations = make([]Toleration, len(in.Tolerations))
		copy(out.Tolerations, in.Tolerations)
	}
}

func (in *NodePoolScalerStatus) DeepCopyInto(out *NodePoolScalerStatus) {
	*out = *in
	if in.LastScaleTime != nil {
		out.LastScaleTime = in.LastScaleTime.DeepCopy()
	}
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

func (in *NodePoolScalerList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *NodePoolScalerList) DeepCopy() *NodePoolScalerList {
	if in == nil {
		return nil
	}
	out := new(NodePoolScalerList)
	in.DeepCopyInto(out)
	return out
}

func (in *NodePoolScalerList) DeepCopyInto(out *NodePoolScalerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]NodePoolScaler, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
