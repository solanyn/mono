package nodepool

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetFieldInt64(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"targetSize": int64(3),
		},
	}}

	val, ok := GetFieldInt64(obj, []string{"spec", "targetSize"})
	if !ok {
		t.Fatal("expected field to be found")
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

func TestGetFieldInt64Missing(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{},
	}}

	_, ok := GetFieldInt64(obj, []string{"spec", "targetSize"})
	if ok {
		t.Error("expected field to not be found")
	}
}

func TestGetFieldInt64DeepPath(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"forProvider": map[string]interface{}{
				"size": int64(5),
			},
		},
	}}

	val, ok := GetFieldInt64(obj, []string{"spec", "forProvider", "size"})
	if !ok {
		t.Fatal("expected field to be found")
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
}

func TestParseFieldPath(t *testing.T) {
	fields := ParseFieldPath("spec.forProvider.targetSize")
	expected := []string{"spec", "forProvider", "targetSize"}
	if len(fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(fields))
	}
	for i, f := range fields {
		if f != expected[i] {
			t.Errorf("field[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestBuildNestedMap(t *testing.T) {
	result := buildNestedMap([]string{"spec", "targetSize"}, int64(3))
	spec, ok := result["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec to be a map")
	}
	val, ok := spec["targetSize"].(int64)
	if !ok {
		t.Fatal("expected targetSize to be int64")
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}
