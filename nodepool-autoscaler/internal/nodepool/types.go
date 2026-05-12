package nodepool

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TargetConfig struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	SizeField []string
}

func (tc TargetConfig) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   tc.Group,
		Version: tc.Version,
		Kind:    tc.Kind,
	}
}

func ParseFieldPath(path string) []string {
	return strings.Split(path, ".")
}

func GetFieldInt64(obj *unstructured.Unstructured, fields []string) (int64, bool) {
	val, found, err := unstructured.NestedInt64(obj.Object, fields...)
	if err != nil || !found {
		return 0, false
	}
	return val, true
}
