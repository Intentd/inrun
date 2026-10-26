package structured

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RawToMap converts a raw informer cache object to map[string]interface{}.
// Works for *unstructured.Unstructured, any typed runtime.Object, and any
// JSON-serializable value. No type assertion required.
//
// Fast path: *unstructured.Unstructured already has the map — returned
// directly with zero allocation. General path: JSON round-trip.
func RawToMap(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil object")
	}
	if u, ok := raw.(*unstructured.Unstructured); ok {
		return u.Object, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("RawToMap: marshal: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("RawToMap: unmarshal: %w", err)
	}
	return result, nil
}

// MetaField extracts a string field from objMap["metadata"].
// Returns "" when the field is absent or not a string.
func MetaField(objMap map[string]interface{}, field string) string {
	meta, _ := objMap["metadata"].(map[string]interface{})
	v, _ := meta[field].(string)
	return v
}
