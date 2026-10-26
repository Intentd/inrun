package structured

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

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

// RenderJSONValue renders a decoded JSON value the way E2EAssertions
// compares: scalars as their text form, null as empty, arrays and objects
// as compact JSON so contains/regex/equals work on their textual shape.
func RenderJSONValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		// JSON numbers decode as float64; render integers without ".0".
		if x == math.Trunc(x) && !math.IsInf(x, 0) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

// LookupPath resolves a dot path against a decoded JSON value. Segments are
// separated by dots; a segment may carry an optional [n] index for arrays
// (e.g. ".kinds[0].crs[0].fields"). A leading dot is ignored. Escaped dots
// (`\.`) are literal dots inside a key, matching SplitField.
//
// Returns (value, true) when the path resolves, (nil, false) when any
// segment is missing or an index is out of range. A resolved null value is
// returned as (nil, true) — the path exists, its value is null.
func LookupPath(v any, path string) (any, bool) {
	segments, ok := pathSegments(path)
	if !ok {
		return nil, false
	}
	cur := v
	for _, seg := range segments {
		key, index := splitIndex(seg)
		if key != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			cur, ok = m[key]
			if !ok {
				return nil, false
			}
		}
		if index != nil {
			arr, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			if *index < 0 || *index >= len(arr) {
				return nil, false
			}
			cur = arr[*index]
		}
	}
	return cur, true
}

// pathSegments splits path on unescaped dots and expands [n] suffixes into
// separate index segments: "kinds[0].crs[0].fields" →
// ["kinds", "[0]", "crs", "[0]", "fields"].
func pathSegments(path string) ([]string, bool) {
	parts := SplitField(strings.TrimPrefix(path, "."))
	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		for {
			open := strings.LastIndexByte(p, '[')
			if open < 0 {
				break
			}
			close := strings.IndexByte(p[open:], ']')
			if close < 0 {
				return nil, false
			}
			close += open
			if open > 0 {
				segments = append(segments, p[:open])
			}
			segments = append(segments, p[open:close+1])
			p = p[close+1:]
		}
		if p != "" {
			segments = append(segments, p)
		}
	}
	return segments, true
}

// splitIndex splits one segment into an optional key and an optional [n]
// index. "[3]" → ("", &3); "fields" → ("fields", nil).
func splitIndex(seg string) (string, *int) {
	if !strings.HasPrefix(seg, "[") || !strings.HasSuffix(seg, "]") {
		return seg, nil
	}
	n, err := strconv.Atoi(seg[1 : len(seg)-1])
	if err != nil {
		return seg, nil
	}
	return "", &n
}
