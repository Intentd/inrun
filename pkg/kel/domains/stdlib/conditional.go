package note

import (
	"fmt"
	"text/template"
)

func conditionalNotes() template.FuncMap {
	return template.FuncMap{
		"ternary":     noteTernary,
		"boolTernary": noteBoolTernary,
		"boolDefault": noteBoolDefault,
		"eqTernary":   noteEqTernary,
		"coalesce":    noteCoalesce,
		"default":     noteDefault,
		"empty":       noteEmpty,
		"notEmpty":    noteNotEmpty,
	}
}

// noteEqTernary returns trueVal when val equals target (string comparison),
// falseVal otherwise. Shorthand for boolTernary (eq val target) trueVal falseVal.
//
//	{{ eqTernary .cross.db.found "true" "ready" "waiting" }}
//	{{ eqTernary .spec.mode "production" "strict" "permissive" }}
func noteEqTernary(val, target, trueVal, falseVal interface{}) interface{} {
	if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", target) {
		return trueVal
	}
	return falseVal
}

// noteTernary returns trueVal when condition is truthy, falseVal otherwise.
// Replaces verbose {{ if }}...{{ else }}...{{ end }} in template expressions.
//
//	{{ ternary .spec.debug "debug" "info" }}
//	{{ ternary .spec.monitoring "enabled" "disabled" }}
func noteTernary(condition interface{}, trueVal, falseVal interface{}) interface{} {
	if noteIsTruthy(condition) {
		return trueVal
	}
	return falseVal
}

// noteCoalesce returns the first non-empty value from a variadic list.
//
//	{{ coalesce .spec.image .spec.defaultImage "nginx:latest" }}
func noteCoalesce(vals ...interface{}) interface{} {
	for _, v := range vals {
		if !noteEmpty(v) {
			return v
		}
	}
	return nil
}

// func noteDefault(dflt, val interface{}) interface{} {
// 	if noteEmpty(val) {
// 		return dflt
// 	}
// 	return val
// }

// noteDefault returns val if non-empty, otherwise returns dflt.
// Both forms are equivalent — the fallback is always the first argument:
//
//	{{ default false .spec.suspend }}          →  false if suspend is absent
//	{{ .spec.suspend | default false }}        →  same call, pipe appends last
//	{{ default 3 .spec.successfulJobsHistoryLimit }}  →  3 if field is absent
func noteDefault(dflt, val interface{}) interface{} {
	if noteEmpty(val) {
		return dflt
	}
	return val
}

// noteEmpty reports whether a value is empty.
// Empty: nil, "", 0, false, empty slice, empty map, "<no value>".
//
//	{{ empty .spec.image }}   →  true when absent
func noteEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == "" || val == "<no value>"
	case bool:
		return !val
	case int:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	}
	return fmt.Sprintf("%v", v) == ""
}

// noteNotEmpty is the inverse of empty.
//
//	{{ notEmpty .spec.image }}   →  true when spec.image is set
func noteNotEmpty(v interface{}) bool {
	return !noteEmpty(v)
}

// func noteIsTruthy(v interface{}) bool {
// 	return !noteEmpty(v)
// }

func noteIsTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int, int8, int16, int32, int64:
		return val != 0
	case uint, uint8, uint16, uint32, uint64:
		return val != 0
	case float32:
		return val != 0.0
	case float64:
		return val != 0.0
	case []interface{}:
		return len(val) > 0
	case map[string]interface{}:
		return len(val) > 0
	default:
		// For any other type (e.g. struct), treat as true
		return true
	}
}

// Boolean fields (like .spec.suspend) should not go through truthiness rules
//
// value: "{{ boolTernary .spec.suspend \"Suspended\" \"Active\" }}"
func noteBoolTernary(cond bool, trueVal, falseVal interface{}) interface{} {
	if cond {
		return trueVal
	}
	return falseVal
}

// Safe boolean default
// {{ boolTernary (boolDefault .spec.suspend false) "Suspended" "Active" }}
func noteBoolDefault(v interface{}, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}
