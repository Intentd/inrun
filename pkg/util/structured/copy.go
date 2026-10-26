package structured

import "k8s.io/apimachinery/pkg/runtime"

// CopyStringMap returns a deep copy of m, or nil when m is empty. Copying
// keeps the snapshot's labels and annotations independent of the source
// object's maps.
func CopyStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// CopySpec returns a deep copy of a spec value, or nil when the value is
// absent or not a map. Deep-copying keeps the snapshot independent of the
// source object, which is pruned immediately after the apply.
func CopySpec(spec interface{}) map[string]interface{} {
	if spec == nil {
		return nil
	}
	if m, ok := spec.(map[string]interface{}); ok {
		return runtime.DeepCopyJSONValue(m).(map[string]interface{})
	}
	return nil
}
