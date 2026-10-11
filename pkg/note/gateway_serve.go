package note

import (
	"text/template"

	"github.com/intentd/inrun/pkg/labels"
)

// serveNotes registers notes that read the gateway serve-provenance annotations
// stamped on every CR the gateway produces.
//
// These let template authors write routing-aware status fields, when conditions,
// and response payloads without hard-coding annotation keys:
//
//	{{ getServeTarget . }}  → "smartapp"
//	{{ getServeAlias . }}   → "public"   (empty when primary target was used)
//	{{ hasServeAlias . }}   → "true"
func serveNotes() template.FuncMap {
	return template.FuncMap{
		"getServeTarget": noteGetServeTarget,
		"getServeAlias":  noteGetServeAlias,
		"hasServeTarget": noteHasServeTarget,
		"hasServeAlias":  noteHasServeAlias,
		"isDirectApply":  noteIsDirectApply,
	}
}

// noteGetServeTarget returns the inrun.dev/serve-target annotation.
// Returns "" when the annotation is absent or when obj is not a CR map.
//
//	{{ getServeTarget . }}  → "smartapp"
func noteGetServeTarget(obj interface{}) string {
	return metaAnnotationField(obj, labels.AnnotationServeTarget)
}

// noteGetServeAlias returns the inrun.dev/serve-alias annotation.
// Returns "" when the CR was submitted directly via its primary target (no alias).
//
//	{{ getServeAlias . }}  → "public"
func noteGetServeAlias(obj interface{}) string {
	return metaAnnotationField(obj, labels.AnnotationServeAlias)
}

// noteHasServeTarget reports whether the serve-target annotation is present and non-empty.
//
//	{{ if hasServeTarget . }}managed by gateway{{ end }}
func noteHasServeTarget(obj interface{}) bool {
	return metaAnnotationField(obj, labels.AnnotationServeTarget) != ""
}

// noteHasServeAlias reports whether the CR was reached via a named alias.
// False when the primary target was used directly.
//
//	{{ if hasServeAlias . }}alias: {{ getServeAlias . }}{{ end }}
func noteHasServeAlias(obj interface{}) bool {
	return metaAnnotationField(obj, labels.AnnotationServeAlias) != ""
}

// noteIsDirectApply reports whether the CR was NOT submitted via the Gateway API —
// i.e. it arrived via kubectl, CI direct apply, or any non-gateway path.
// Returns true only when none of the three provenance annotations are present.
//
//	{{ isDirectApply . }}  → true when serve-target, serve-alias, and serve-source are all absent
func noteIsDirectApply(obj interface{}) bool {
	return !noteHasServeTarget(obj) && !noteHasServeAlias(obj)
}

// metaAnnotationField reads a single annotation value from metadata.annotations.
// Returns "" when obj is not a map, metadata is absent, or the key is not set.
func metaAnnotationField(obj interface{}, key string) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	meta, ok := m["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	ann, ok := meta["annotations"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := ann[key].(string)
	return v
}
