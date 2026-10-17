package note

import (
	"strings"
	"text/template"
)

// eventNotes registers helpers for inspecting Warning events embedded under
// _warnings by the enrichment layer. All functions are pure — they navigate
// the _warnings slice already present in the object map and perform no I/O.
//
// Usage:
//
//	tmpl.Funcs(note.eventNotes())
//
// Template examples:
//
//	{{ hasWarnings .children.deployment }}
//	{{ warningCount .children.deployment }}
//	{{ firstWarning .children.deployment }}
//	{{ warningMessages .children.deployment }}
//	{{ warningReasons .children.deployment }}
//
// All require enrich: [events] (or enrichAll: true) on the CRD.
// Without enrichment _warnings is absent and all functions return zero values.
func eventNotes() template.FuncMap {
	return template.FuncMap{
		"hasWarnings":     noteHasWarnings,
		"warningCount":    noteWarningCount,
		"firstWarning":    noteFirstWarning,
		"warningMessages": noteWarningMessages,
		"warningReasons":  noteWarningReasons,
	}
}

// ── Warning event notes ───────────────────────────────────────────────────────

// noteHasWarnings returns true when _warnings contains at least one event.
// Requires enrich: [events] on the CRD.
//
//	{{ hasWarnings .children.deployment }}
func noteHasWarnings(obj interface{}) bool {
	return len(getWarnings(obj)) > 0
}

// noteWarningCount returns the number of warning events in _warnings.
// Requires enrich: [events] on the CRD.
//
//	{{ warningCount .children.deployment }}  → 3
func noteWarningCount(obj interface{}) int {
	return len(getWarnings(obj))
}

// noteFirstWarning returns the message of the first warning event.
// Returns "" when there are no warnings.
// Requires enrich: [events] on the CRD.
//
//	{{ firstWarning .children.deployment }}
//	→ "Back-off restarting failed container"
func noteFirstWarning(obj interface{}) string {
	ws := getWarnings(obj)
	if len(ws) == 0 {
		return ""
	}
	w, ok := ws[0].(map[string]interface{})
	if !ok {
		return ""
	}
	msg, _ := w["message"].(string)
	return msg
}

// noteWarningMessages returns all warning messages as a comma-separated string.
// Requires enrich: [events] on the CRD.
//
//	{{ warningMessages .children.deployment }}
//	→ "Back-off restarting failed container, Liveness probe failed"
func noteWarningMessages(obj interface{}) string {
	return joinWarningField(obj, "message")
}

// noteWarningReasons returns all warning reasons as a comma-separated string.
// Requires enrich: [events] on the CRD.
//
//	{{ warningReasons .children.deployment }}
//	→ "BackOff, Unhealthy"
func noteWarningReasons(obj interface{}) string {
	return joinWarningField(obj, "reason")
}

func getWarnings(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	ws, _ := m["_warnings"].([]interface{})
	return ws
}

func joinWarningField(obj interface{}, field string) string {
	var parts []string
	for _, w := range getWarnings(obj) {
		wm, ok := w.(map[string]interface{})
		if !ok {
			continue
		}
		v, _ := wm[field].(string)
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}
