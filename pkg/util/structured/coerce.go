package structured

import (
	"encoding/json"
	"strconv"
	"strings"
)

// TryCoerceString attempts to parse a resolved template string as a native
// Go type, so integer/boolean/JSON CRD fields pass Kubernetes API server
// validation instead of being submitted as literal strings. Returns
// float64 for integers and floats (JSON-safe), bool for booleans,
// map[string]any/[]any for a JSON object/array, and the original string
// for everything else — including a malformed near-JSON value, so callers
// can fail closed the same way the numeric/boolean attempts already do.
func TryCoerceString(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return float64(i)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	if trimmed := strings.TrimSpace(s); len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var v any
		if json.Unmarshal([]byte(trimmed), &v) == nil {
			return v
		}
	}
	return s
}

// ResolveArgsMap walks rawArgs and evaluates any string values that contain
// Go template expressions using eval. Resolved strings are coerced with
// TryCoerceString so bool/int/float/JSON values arrive as their native types.
// Non-string and non-template values pass through unchanged.
// Nested maps are walked recursively.
func ResolveArgsMap(rawArgs map[string]interface{}, eval func(string) (string, bool)) map[string]interface{} {
	out := make(map[string]interface{}, len(rawArgs))
	for k, v := range rawArgs {
		out[k] = resolveArgValue(v, eval)
	}
	return out
}

func resolveArgValue(v interface{}, eval func(string) (string, bool)) interface{} {
	switch val := v.(type) {
	case string:
		if !strings.Contains(val, "{{") {
			return val
		}
		if resolved, ok := eval(val); ok {
			return TryCoerceString(resolved)
		}
		return val
	case map[string]interface{}:
		sub := make(map[string]interface{}, len(val))
		for k, sv := range val {
			sub[k] = resolveArgValue(sv, eval)
		}
		return sub
	default:
		return v
	}
}
