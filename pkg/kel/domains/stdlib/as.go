package note

import (
	"encoding/json"
	"text/template"

	"gopkg.in/yaml.v3"
)

func asNotes() template.FuncMap {
	return template.FuncMap{
		"asList":   asList,
		"asMap":    asMap,
		"asString": asString,
	}
}

// asList converts any input into a []any.
// Supported inputs:
//   - []any (returned as-is)
//   - YAML list (string)
//   - JSON array (string)
//   - anything else → empty list
func asList(input any) []any {
	switch v := input.(type) {

	case []any:
		return v

	case string:
		// Try YAML
		var y any
		if err := yaml.Unmarshal([]byte(v), &y); err == nil {
			if list, ok := y.([]any); ok {
				return list
			}
		}

		// Try JSON
		var j []any
		if err := json.Unmarshal([]byte(v), &j); err == nil {
			return j
		}

		return []any{}

	default:
		return []any{}
	}
}

// asMap converts any input into a map[string]any.
// Supported inputs:
//   - map[string]any (returned as-is)
//   - YAML map (string)
//   - JSON object (string)
//   - anything else → empty map
func asMap(input any) map[string]any {
	switch v := input.(type) {

	case map[string]any:
		return v

	case string:
		// Try YAML
		var y any
		if err := yaml.Unmarshal([]byte(v), &y); err == nil {
			if m, ok := y.(map[string]any); ok {
				return m
			}
		}

		// Try JSON
		var j map[string]any
		if err := json.Unmarshal([]byte(v), &j); err == nil {
			return j
		}

		return map[string]any{}

	default:
		return map[string]any{}
	}
}

// asString converts any input into a string.
// Supported inputs:
//   - string (returned as-is)
//   - YAML scalar (string)
//   - JSON scalar (string)
//   - anything else → ""
func asString(input any) string {
	switch v := input.(type) {

	case string:
		return v

	case []byte:
		return string(v)

	default:
		// Try JSON encode → string
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return ""
	}
}
