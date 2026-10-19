package shared

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GVKFromFields parses apiVersion and kind into a GroupVersionKind.
func GVKFromFields(apiVersion, kind string) (schema.GroupVersionKind, error) {
	if apiVersion == "" || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion and kind are required")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return gv.WithKind(kind), nil
}

// ToJSONSafe converts integer types to float64, recursively, so a value from
// YAML can be set on an unstructured object: unstructured.DeepCopyJSONValue
// accepts float64 only.
func ToJSONSafe(v any) any {
	switch vv := v.(type) {
	case int:
		return float64(vv)
	case int32:
		return float64(vv)
	case int64:
		return float64(vv)
	case uint:
		return float64(vv)
	case uint32:
		return float64(vv)
	case uint64:
		return float64(vv)
	case map[string]any:
		out := make(map[string]any, len(vv))
		for k, val := range vv {
			out[k] = ToJSONSafe(val)
		}
		return out
	case []any:
		out := make([]any, len(vv))
		for i, e := range vv {
			out[i] = ToJSONSafe(e)
		}
		return out
	default:
		return vv
	}
}
