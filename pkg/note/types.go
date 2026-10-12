package note

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

func typeNotes() template.FuncMap {
	return template.FuncMap{
		"toInt":    noteToInt,
		"toFloat":  noteToFloat,
		"toBool":   noteToBool,
		"toString": noteToString,
		"toJson":   noteToJson,
		"toList":   toList,

		// typeOf v — returns the type name as a string
		// "string", "number", "bool", "map", "slice", "null", "unknown"
		"typeOf": TypeOf,

		// len v — returns element count or string length
		// Overrides the built-in template len with one that handles maps
		"len": InrunLen,

		// Shorthand type checks
		"typeMap":     typeMap,
		"typeList":    typeList,
		"typeString":  typeString,
		"typeNumber":  typeNumber,
		"typeBool":    typeBool,
		"typeNull":    typeNull,
		"isEmpty":     isEmpty,
		"isScalar":    isScalar,
		"isComposite": isComposite,
	}
}

// noteToInt converts any value to int64.
//
//	{{ toInt "3" }}    →  3
//	{{ toInt 3.7 }}    →  3   (truncates)
//	{{ toInt true }}   →  1
func noteToInt(v interface{}) (int64, error) {
	if b, ok := v.(bool); ok {
		if b {
			return 1, nil
		}
		return 0, nil
	}
	f, err := anyToFloat(v)
	if err != nil {
		return 0, fmt.Errorf("toInt: %w", err)
	}
	return int64(f), nil
}

// noteToFloat converts any value to float64.
func noteToFloat(v interface{}) (float64, error) {
	return anyToFloat(v)
}

// noteToBool converts a value to bool.
// Truthy: true, 1, "true", "yes", "on", "1", "True", "TRUE", "YES".
// Falsy: false, 0, "", "false", "no", "off", "0", "False", "FALSE".
//
//	{{ toBool "yes" }}   →  true
//	{{ toBool 1 }}       →  true
//	{{ toBool "" }}      →  false
func noteToBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case int, int32, int64, float32, float64:
		f, _ := anyToFloat(v)
		return f != 0, nil
	case string:
		switch val {
		case "true", "True", "TRUE", "1", "yes", "YES", "Yes", "on", "ON":
			return true, nil
		case "false", "False", "FALSE", "0", "no", "NO", "No", "off", "OFF", "":
			return false, nil
		}
		return false, fmt.Errorf("toBool: unrecognised value %q", val)
	}
	return false, fmt.Errorf("toBool: cannot convert %T", v)
}

// noteToString converts any value to its string representation.
//
//	{{ toString 42 }}       →  "42"
//	{{ toString true }}     →  "true"
//	{{ toString 3.14 }}     →  "3.14"
func noteToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// noteToJson converts any value to its JSON string representation.
// Returns an error if marshaling fails (e.g., channel, function).
//
//	{{ toJson .spec }}                →  `{"replicas":3,"enabled":true}`
//	{{ toJson .children.replicaset }} →  full ReplicaSet object as JSON
func noteToJson(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("toJson: %w", err)
	}
	return string(b), nil
}

// toList converts a comma-separated string to a trimmed, de-duplicated slice.
//
// Designed for use in serve.view.exclude and anywhere else a template
// expression needs to produce a list from a single string — annotations,
// labels, or literal values.
//
// Usage in a Catalog:
//
//	exclude: '{{ toList (getAnnotation . "platform.myorg.io/exclude") }}'
//	exclude: '{{ toList "metadata.managedFields,status.observedGeneration" }}'
//
// Returns an empty slice when the input is empty or blank. Whitespace around
// each entry is trimmed. Blank entries (e.g. trailing comma) are dropped.
func toList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// typeOf — returns the runtime type name of any value.
// inrunLen — returns the length of a string, slice, or map.
//
// typeOf is used in:
//   - Template expressions: {{ typeOf .spec.schedule }} → "string" or "map"
//   - when: conditions:    operator: typeOf, value: map
//
// The condition operator path uses NavigateRawPath → note.TypeOf directly.
// The template expression path uses the FuncMap entry registered in note.Map().
// Both must return the same strings for the Catalog to be predictable.
//
// Type strings (match Python/JavaScript convention for familiarity):
//
//	"string"  → Go string
//	"number"  → Go float64, int64, int (JSON numbers come back as float64)
//	"bool"    → Go bool
//	"map"     → Go map[string]interface{} (YAML objects, structured fields)
//	"slice"   → Go []interface{} (YAML arrays, list fields)
//	"null"    → nil
//	"unknown" → any other type
//
// YAML type behaviour:
//
//	spec:
//	  schedule: "*/5 * * * *"          → typeOf returns "string"
//	  schedule:                          → typeOf returns "map"
//	    minute: "*/5"
//	  regions: [us-east-1, eu-west-1]   → typeOf returns "slice"
//	  replicas: 3                        → typeOf returns "number"
//	  enabled: true                      → typeOf returns "bool"
//
// TypeOf returns the type name of any interface{} value.
// Exported so pkg/types/when.go can call it from EvaluateOneCond
// without the template engine being involved.
func TypeOf(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64, float32:
		return "number"
	case int, int32, int64, uint, uint32, uint64:
		return "number"
	case bool:
		return "bool"
	case map[string]interface{}:
		return "map"
	case []interface{}:
		return "slice"
	default:
		return "unknown"
	}
}

// InrunLen returns the length of a string, slice, or map.
// Named inrunLen to avoid shadowing Go's built-in len.
// Registered in note.Map() as "len" since Go templates do not
// have a built-in len that handles all three types uniformly.
//
// Template: {{ len .spec.regions }}          → 3 (slice with 3 elements)
// Template: {{ len .spec.schedule }}         → 5 (map with 5 fields)
// Template: {{ len .metadata.name }}         → 12 (string length)
func InrunLen(v interface{}) int {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case string:
		return len(t)
	case []interface{}:
		return len(t)
	case map[string]interface{}:
		return len(t)
	default:
		return 0
	}
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case []interface{}:
		return len(t) == 0
	case map[string]interface{}:
		return len(t) == 0
	default:
		return false
	}
}

func isScalar(v interface{}) bool {
	switch TypeOf(v) {
	case "string", "number", "bool":
		return true
	default:
		return false
	}
}

func isComposite(v interface{}) bool {
	switch TypeOf(v) {
	case "map", "slice":
		return true
	default:
		return false
	}
}

// Shothand helpers
func typeMap(v interface{}) bool {
	return TypeOf(v) == "map"
}

func typeList(v interface{}) bool {
	return TypeOf(v) == "slice"
}

func typeString(v interface{}) bool {
	return TypeOf(v) == "string"
}

func typeNumber(v interface{}) bool {
	return TypeOf(v) == "number"
}

func typeBool(v interface{}) bool {
	return TypeOf(v) == "bool"
}

func typeNull(v interface{}) bool {
	return TypeOf(v) == "null"
}
