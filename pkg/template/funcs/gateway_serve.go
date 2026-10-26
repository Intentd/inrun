package funcs

import (
	"text/template"

	"github.com/intentd/inrun/pkg/labels"
)

// serveFuncs — Serve
//
// Domain: serve
//
// Read the provenance a CR carries when Inrun's gateway created it: the serve
// target and alias it came through, and whether it was applied directly
// instead.
func serveFuncs() template.FuncMap {
	return template.FuncMap{
		"getServeTarget": getServeTarget,
		"getServeAlias":  getServeAlias,
		"hasServeTarget": hasServeTarget,
		"hasServeAlias":  hasServeAlias,
		"isDirectApply":  isDirectApply,
	}
}

// getServeTarget returns the `inrun.dev/serve-target` annotation — the serve
// target name used when the CR was submitted. Returns `""` when the annotation
// is absent (CR was not submitted via the Gateway API).
//
// Keywords: serve, gateway, target, annotation, provenance, routing, string
//
// Example:
//
//	# value: "{{ getServeTarget . }}"  → "smartapp"
//
//	# Expose the target in a status field:
//	- path: serveTarget
//	  value: "{{ getServeTarget . }}"
func getServeTarget(obj interface{}) string {
	return metaAnnotationField(obj, labels.AnnotationServeTarget)
}

// getServeAlias returns the `inrun.dev/serve-alias` annotation — the alias
// name used when the CR was submitted, if any. Returns `""` when the primary
// target was used directly (not an alias).
//
// Keywords: serve, gateway, alias, annotation, provenance, routing, string
//
// Example:
//
//	# value: "{{ getServeAlias . }}"  → "public"   (empty when primary target used)
//
//	# Expose the alias alongside the target:
//	- path: serveAlias
//	  value: "{{ getServeAlias . }}"
func getServeAlias(obj interface{}) string {
	return metaAnnotationField(obj, labels.AnnotationServeAlias)
}

// hasServeTarget reports whether the `inrun.dev/serve-target` annotation is
// present and non-empty. Returns `true` for any CR submitted via the Gateway
// API.
//
// Keywords: serve, gateway, target, annotation, boolean, presence
//
// Example:
//
//	# value: "{{ hasServeTarget . }}"  → "true"
//
//	# Conditional block in a response payload:
//	when:
//	  - field: "{{ hasServeTarget . }}"
//	    equals: "true"
func hasServeTarget(obj interface{}) bool {
	return metaAnnotationField(obj, labels.AnnotationServeTarget) != ""
}

// hasServeAlias reports whether the `inrun.dev/serve-alias` annotation is set
// — i.e., the CR was reached via a named alias rather than the primary target.
//
// Keywords: serve, gateway, alias, annotation, boolean, routing
//
// Example:
//
//	# value: "{{ hasServeAlias . }}"  → "true"
//
//	# Conditional display in a status field:
//	- path: routedVia
//	  value: "{{ if hasServeAlias . }}alias:{{ getServeAlias . }}{{ else }}direct{{ end }}"
func hasServeAlias(obj interface{}) bool {
	return metaAnnotationField(obj, labels.AnnotationServeAlias) != ""
}

// isDirectApply reports whether the CR was NOT submitted via the Gateway API —
// i.e. it arrived via `kubectl`, CI direct apply, or any other non-gateway
// path. Returns `true` only when none of the three provenance annotations
// (`serve-target`, `serve-alias`, `serve-source`) are present. Use this to
// gate stricter validation rules, different mutation defaults, or reconciler
// branches that handle raw applies separately from gateway-classified intent.
//
// Keywords: serve, gateway, direct, kubectl, annotation, boolean, provenance
//
// Example:
//
//	# value: "{{ isDirectApply . }}"  → "true" when no serve-target annotation is present
//
//	# Enforce stricter image policy for direct applies:
//	validation:
//	  rules:
//	    - field: spec.image
//	      prefix: "myorg/"
//	      message: "direct applies must use the internal registry"
//	      action: deny
//	      when:
//	        - field: "{{ isDirectApply . }}"
//	          equals: "true"
func isDirectApply(obj interface{}) bool {
	return !hasServeTarget(obj) && !hasServeAlias(obj)
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
