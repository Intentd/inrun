package note

import "text/template"

// safeAccessNotes registers safe access helpers.
//
// Usage:
//
//	tmpl.Funcs(note.safeAccessNotes())
//
// Template examples:
//
//	{{ getOr .spec.replicas 1 }}
//	{{ getStringOr (get . "spec" "image") "nginx:latest" }}
//	{{ getIntOr (get . "spec" "replicas") 1 }}
func safeAccessNotes() template.FuncMap {
	return template.FuncMap{
		"getOr":       noteGetOr,
		"getStringOr": noteGetStringOr,
		"getIntOr":    noteGetIntOr,
		"getBoolOr":   noteGetBoolOr,
	}
}

// noteGetOr returns val if non-empty, otherwise def.
//
//	{{ getOr .spec.replicas 1 }}
func noteGetOr(val interface{}, def interface{}) interface{} {
	if noteEmpty(val) {
		return def
	}
	return val
}

// noteGetStringOr returns val as string or def.
//
//	{{ getStringOr (get . "spec" "image") "nginx:latest" }}
func noteGetStringOr(val interface{}, def string) string {
	if s, ok := val.(string); ok && s != "" {
		return s
	}
	return def
}

// noteGetIntOr returns val as int or def.
//
//	{{ getIntOr (get . "spec" "replicas") 1 }}
func noteGetIntOr(val interface{}, def int) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}

// noteGetBoolOr returns val as bool or def.
//
//	{{ getBoolOr (get . "spec" "enabled") false }}
func noteGetBoolOr(val interface{}, def bool) bool {
	if b, ok := val.(bool); ok {
		return b
	}
	return def
}
