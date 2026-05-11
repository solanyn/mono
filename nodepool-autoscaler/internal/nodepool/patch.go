package nodepool

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PatchTargetSize(ctx context.Context, c client.Client, tc TargetConfig, targetSize int64) error {
	patch := buildNestedMap(tc.SizeField, targetSize)

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshaling patch: %w", err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(tc.GVK())
	obj.SetName(tc.Name)
	obj.SetNamespace(tc.Namespace)

	return c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patchBytes))
}

func buildNestedMap(fields []string, value interface{}) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	if len(fields) == 1 {
		return map[string]interface{}{fields[0]: value}
	}
	return map[string]interface{}{
		fields[0]: buildNestedMap(fields[1:], value),
	}
}
