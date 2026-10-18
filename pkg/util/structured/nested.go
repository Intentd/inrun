package structured

import (
	"fmt"
	"strings"
)

// NestedSlice navigates a map[string]interface{} using dot-notation keys
// and returns the final slice value.
// Returns nil, false if any key in the path is missing or not a map.
func NestedSlice(obj map[string]interface{}, keys ...string) ([]interface{}, bool) {
	cur := obj
	for i, k := range keys {
		if i == len(keys)-1 {
			v, ok := cur[k].([]interface{})
			return v, ok
		}
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// NestedMap navigates a map[string]interface{} using dot-notation keys
// and returns the final map value.
// Returns nil, false if any key in the path is missing or not a map.
func NestedMap(obj map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	if len(keys) == 0 {
		return nil, false
	}
	cur := obj
	for _, k := range keys {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// DeepCopyMap returns a shallow-to-one-level deep copy of a map[string]interface{}.
// Nested maps are also copied; slices and scalar values share the same pointer.
// Sufficient for our use case: we only modify top-level keys and nested map keys
// via deleteNestedPath — we never mutate slice elements or scalar values.
func DeepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		if nested, ok := v.(map[string]interface{}); ok {
			dst[k] = DeepCopyMap(nested)
		} else {
			dst[k] = v
		}
	}
	return dst
}

// SetNestedPath sets a value at a dot-notation path (see SplitField), creating
// intermediate maps. It errors when a segment on the way is not a map.
func SetNestedPath(m map[string]interface{}, path string, value interface{}) error {
	parts := SplitField(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	// Navigate to the parent
	current := m
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		if _, ok := current[key]; !ok {
			current[key] = make(map[string]interface{})
		}
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return fmt.Errorf("path segment %q is not a map (cannot set nested value)", key)
		}
		current = next
	}

	// Set the value at the final key
	current[parts[len(parts)-1]] = value
	return nil
}

// GetNestedPath returns the value at a dot-notation path (see SplitField), and
// whether it exists.
func GetNestedPath(m map[string]interface{}, path string) (interface{}, bool) {
	parts := SplitField(path)
	if len(parts) == 0 {
		return nil, false
	}
	current := m
	for i, key := range parts {
		val, ok := current[key]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return val, true
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

// DeleteNestedPath removes the key at a dot-notation path (see SplitField) in
// place. A path that does not exist is not an error.
func DeleteNestedPath(obj map[string]interface{}, path string) {
	parts := SplitField(path)
	current := obj
	for i, key := range parts {
		if current == nil {
			return
		}
		if i == len(parts)-1 {
			delete(current, key)
			return
		}
		current, _ = current[key].(map[string]interface{})
	}
}

// IsNestedPath returns true if the path contains a dot.
func IsNestedPath(path string) bool {
	return strings.Contains(path, ".")
}

// SplitPath is SplitField without empty segments.
func SplitPath(path string) []string {
	var parts []string
	for _, p := range SplitField(path) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// SetFieldPath sets a value at a dot-notation path (see SplitField), replacing
// anything on the way that is not a map.
func SetFieldPath(obj map[string]interface{}, path string, value interface{}) {
	parts := SplitField(path)
	current := obj

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		if _, ok := current[part]; !ok {
			current[part] = map[string]interface{}{}
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			current[part] = next
		}
		current = next
	}
}
