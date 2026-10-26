package children

import "github.com/intentd/inrun/pkg/types"

// resolveEnrichmentTarget returns the canonical built-in name for a given
// enrichment identifier (name, plural, shorthand, alias). Returns "" if unknown.
func resolveEnrichmentTarget(s string) string {
	return enrichmentIndex[s]
}

// enrichmentEnabled reports whether the CRD has enabled enrichment for the
// given identifier. Exact match is checked first — this is the common case
// and handles synthetic keys (owner, backingpods, pvcs, storageclass, etc.)
// that have no Kubernetes built-in registration.
//
// If no exact match, the identifier is resolved to its canonical built-in name
// and all equivalent identifiers (plural, shorthands, aliases) are checked.
// This allows users to write "cj" instead of "cronjob" or "hpas" instead of
// "horizontalpodautoscaler" and still match enrichers that check by canonical.
func enrichmentEnabled(s string, crd types.CRDEntry) bool {
	// Fast path — user wrote exactly what the enricher checks (most common case).
	if crd.ShouldEnrich(s) {
		return true
	}
	// Normalise and check all equivalent identifiers for the canonical target.
	canonical, ok := enrichmentIndex[s]
	if !ok {
		return false
	}
	for _, alias := range enrichmentGroups[canonical] {
		if crd.ShouldEnrich(alias) {
			return true
		}
	}
	return false
}
