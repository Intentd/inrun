package structured

import "encoding/json"

// JSONNormalize round-trips a value through encoding/json, normalising all
// numeric types to float64. This ensures values from the Kubernetes API
// (int64 for integer CRD fields) compare correctly with values decoded from
// annotation JSON strings (float64 via encoding/json).
func JSONNormalize(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	err = json.Unmarshal(b, &out)
	if err != nil {
		return v
	}
	return out
}

// ToJSONSafe recursively converts Go values to JSON-safe types.
// YAML unmarshalling produces int/int64 for integers, but k8s DeepCopyJSONValue
// only handles float64. This normalises the tree before any SetNestedField call.
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
