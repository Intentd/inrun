package types

import "strings"

//
// ────────────────────────────────────────────────────────────────────────────────
//   CROSS‑FIELD PARSING HELPERS
//   These helpers parse field paths of the form:
//
//       cross.<crd>.<category>.<field>
//
//   Examples:
//       cross.loader.metrics.queueDepth
//       cross.db.health.lastError
//       cross.api.info.status.phase
//       cross.worker.events.items[0].message
//
//   They are used by the resolver, autoscaler, and ONCOP router.
// ────────────────────────────────────────────────────────────────────────────────
//

// ExtractCrossCRD returns the <crd> portion of a cross.* field path.
// Examples:
//
//	cross.loader.metrics.queueDepth      → "loader"
//	cross.processor.health.state         → "processor"
//	cross.db.info.status.phase           → "db"
func ExtractCrossCRD(field string) string {
	if !strings.HasPrefix(field, "cross.") {
		return ""
	}
	rest := strings.TrimPrefix(field, "cross.")
	dot := strings.Index(rest, ".")
	if dot < 0 {
		return ""
	}
	return rest[:dot]
}

// ExtractCrossSuffix returns the suffix after cross.<crd>. in a field path.
// Examples:
//
//	cross.loader.metrics.queueDepth  → "metrics.queueDepth"
//	cross.db.health.lastError        → "health.lastError"
//	cross.api.info.status.phase      → "info.status.phase"
func ExtractCrossSuffix(field string) string {
	if !strings.HasPrefix(field, "cross.") {
		return ""
	}
	rest := strings.TrimPrefix(field, "cross.")
	dot := strings.Index(rest, ".")
	if dot < 0 {
		return ""
	}
	return rest[dot+1:]
}

// ExtractCrossCategory returns the category segment after cross.<crd>.
// Examples:
//
//	cross.loader.metrics.queueDepth → "metrics"
//	cross.db.health.state           → "health"
//	cross.api.info.status.phase     → "info"
func ExtractCrossCategory(field string) string {
	suffix := ExtractCrossSuffix(field)
	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return ""
	}
	return suffix[:dot]
}

// ExtractCrossFieldName returns the field name after the category.
// Examples:
//
//	cross.loader.metrics.queueDepth → "queueDepth"
//	cross.db.health.lastError       → "lastError"
//	cross.api.info.status.phase     → "status.phase"
func ExtractCrossFieldName(field string) string {
	suffix := ExtractCrossSuffix(field)
	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return ""
	}
	return suffix[dot+1:]
}

// ExtractCrossNamespace returns the namespace portion for ONCOP info/events
// when encoded in a field path of the form:
//
//	cross.<crd>.info.<namespace>.<name>.status.phase
//
// If no namespace is encoded, returns "".
//
// Examples:
//
//	cross.db.info.default.my-db.status.phase → "default"
//	cross.api.info.prod.service-a.spec.port  → "prod"
//	cross.loader.metrics.queueDepth          → ""
//
// NOTE: Namespace is only encoded for info/events categories. Metrics/health
// do not carry namespace in the field path.
func ExtractCrossNamespace(field string) string {
	category := ExtractCrossCategory(field)
	if category != "info" && category != "events" {
		return ""
	}

	suffix := ExtractCrossSuffix(field)
	parts := strings.Split(suffix, ".")
	if len(parts) < 3 {
		return ""
	}

	// suffix = "<category>.<namespace>.<name>...."
	// parts[0] = category
	// parts[1] = namespace
	return parts[1]
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   PARSED STRUCT
// ────────────────────────────────────────────────────────────────────────────────
//

// CrossField describes a parsed cross.<crd>.<category>.<field> reference.
type CrossField struct {
	CRD       string // loader, db, processor, etc.
	Category  string // metrics, health, info, events, children
	Namespace string // optional (info/events only)
	Field     string // queueDepth, lastError, status.phase, etc.
}

// ParseCrossField parses a cross.* field path into a structured CrossField.
//
// Examples:
//
//	cross.loader.metrics.queueDepth
//	  → CRD=loader, Category=metrics, Namespace="", Field="queueDepth"
//
//	cross.db.health.lastError
//	  → CRD=db, Category=health, Namespace="", Field="lastError"
//
//	cross.api.info.default.my-api.status.phase
//	  → CRD=api, Category=info, Namespace=default, Field="status.phase"
//
// Returns nil if the field is not a valid cross.* reference.
func ParseCrossField(field string) *CrossField {
	if !strings.HasPrefix(field, "cross.") {
		return nil
	}

	crd := ExtractCrossCRD(field)
	if crd == "" {
		return nil
	}

	category := ExtractCrossCategory(field)
	if category == "" {
		return nil
	}

	ns := ExtractCrossNamespace(field)
	name := ExtractCrossFieldName(field)

	return &CrossField{
		CRD:       crd,
		Category:  category,
		Namespace: ns,
		Field:     name,
	}
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   CATEGORY DETECTORS
// ────────────────────────────────────────────────────────────────────────────────
//

// IsCrossMetricField returns true when the field path refers to another
// operatorbox's runtime metrics via the cross.*.metrics.* namespace.
func IsCrossMetricField(field string) bool {
	return strings.HasPrefix(field, "cross.") &&
		strings.Contains(field, ".metrics.")
}

// IsCrossHealthField returns true when the field path refers to another
// operatorbox's runtime health via the cross.*.health.* namespace.
func IsCrossHealthField(field string) bool {
	return strings.HasPrefix(field, "cross.") &&
		strings.Contains(field, ".health.")
}

// IsCrossInfoField returns true when the field path refers to another
// operatorbox's CR detail (the CRD info endpoint) via cross.*.info.*.
func IsCrossInfoField(field string) bool {
	return strings.HasPrefix(field, "cross.") &&
		strings.Contains(field, ".info.")
}

// IsCrossEventsField returns true when the field path refers to another
// operatorbox's CR events via the cross.*.events.* namespace.
func IsCrossEventsField(field string) bool {
	return strings.HasPrefix(field, "cross.") &&
		strings.Contains(field, ".events.")
}

// IsCrossChildrenField returns true when the field path refers to another
// operatorbox's managed Kubernetes children via cross.*.children.*.
func IsCrossChildrenField(field string) bool {
	return strings.HasPrefix(field, "cross.") &&
		strings.Contains(field, ".children.")
}
