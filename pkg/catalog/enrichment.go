// pkg/catalog/enrichment.go
package catalog

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// EnrichCRDEntry checks whether a CRD entry uses kind-only declaration
// and, if so, enriches it with the corresponding built-in API metadata.
//
// Called during Catalog validation before the CRD entry is used at runtime.
//
// A CRD entry qualifies for enrichment when:
//   - apiTypes.kind is set
//   - apiTypes.group is empty
//   - apiTypes.version is empty
//   - apiTypes.plural is empty
//
// All three must be empty for enrichment to trigger — a partially-specified
// entry (e.g. kind + group but no version) is an error, not an enrichment
// candidate. This prevents silent misconfigurations.
//
// Returns:
//   - EnrichmentNotNeeded: entry was already fully specified, no change
//   - EnrichmentApplied:   entry was enriched successfully
//   - EnrichmentFailed:    kind not found in built-in registry, error returned
func EnrichCRDEntry(entry *types.CRDEntry) (types.EnrichmentOutcome, error) {
	apiTypes := &entry.APITypes

	// Already fully specified — nothing to do
	if isFullySpecified(entry) {
		return types.EnrichmentNotNeeded, nil
	}

	// Partially specified — user declared some but not all required fields.
	// This is a misconfiguration, not a kind-only declaration.
	// if isPartiallySpecified(apiTypes) {
	// 	return types.EnrichmentFailed, fmt.Errorf(
	// 		"CRD %q: apiTypes is partially specified — either declare kind only "+
	// 			"(for Kubernetes built-ins) or declare all fields "+
	// 			"(kind, group, version).\n"+
	// 			"  Have: kind=%q group=%q version=%q\n"+
	// 			"  Hint: for built-ins, only kind is needed:\n"+
	// 			"    apiTypes:\n"+
	// 			"      kind: %s",
	// 		entry.Name,
	// 		apiTypes.Kind, apiTypes.Group, apiTypes.Version,
	// 		apiTypes.Kind,
	// 	)
	// }

	// Kind-only declaration — attempt built-in lookup
	if apiTypes.Kind == "" {
		return types.EnrichmentFailed, fmt.Errorf(
			"CRD %q: apiTypes.kind is required", entry.Name,
		)
	}

	result := children.LookupBuiltIn(apiTypes.Kind)
	if !result.Found() {
		return types.EnrichmentFailed, fmt.Errorf(
			"CRD %q: kind %q is not a known Kubernetes built-in and apiTypes "+
				"is incomplete (missing group, version, plural).\n\n"+
				"  For Kubernetes built-ins, use the kind name only:\n"+
				"    apiTypes:\n"+
				"      kind: Deployment\n\n"+
				"  For custom CRDs, declare all fields:\n"+
				"    apiTypes:\n"+
				"      kind: %s\n"+
				"      group: your.group.io\n"+
				"      version: v1alpha1\n"+
				"      plural: %ss\n\n"+
				"  Supported built-in kinds: %s",
			entry.Name,
			apiTypes.Kind,
			apiTypes.Kind,
			strings.ToLower(apiTypes.Kind),
			strings.Join(children.AllBuiltInKinds(), ", "),
		)
	}

	// Apply enrichment
	IsNamespaced := true
	if !result.Namespaced() {
		IsNamespaced = false
	}

	apiTypes.Kind = result.Kind()
	apiTypes.Group = result.Group()
	apiTypes.Version = result.Version()
	apiTypes.Plural = result.Plural()
	apiTypes.APIPath = result.APIPath()
	entry.Namespaced = &IsNamespaced

	// Mark as a built-in for informational logging and inrun validate output
	entry.IsBuiltIn = true
	entry.BuiltInGroup = result.DisplayGroup()

	logger.Debug().
		Str("crd", entry.Name).
		Str("kind", result.Kind()).
		Str("group", result.DisplayGroup()).
		Str("version", result.Version()).
		Str("plural", result.Plural()).
		Msg("built-in: enriched kind-only declaration")

	return types.EnrichmentApplied, nil
}

// isFullySpecified reports whether all required apiTypes fields are present.
func isFullySpecified(entry *types.CRDEntry) bool {
	a := &entry.APITypes
	if entry.IsBuiltInType() {
		return a.Kind != "" && a.Version != "" && a.Plural != ""
		// Group is intentionally excluded — core group is legitimately empty
	} else {
		return a.Kind != "" && a.Group != "" && a.Version != ""
		// Plural is intentailly excluded — optional
	}
}

// isPartiallySpecified reports whether some but not all required fields are set.
// Kind-only declaration is NOT partial — it is the built-in shorthand.
func isPartiallySpecified(a *types.APITypes) bool {
	if a.Kind == "" {
		return false // no kind at all — caught elsewhere
	}
	// If any of these are set, all must be set
	hasGroup := a.Group != ""
	hasVersion := a.Version != ""
	hasPlural := a.Plural != ""

	setCount := 0
	if hasGroup {
		setCount++
	}
	if hasVersion {
		setCount++
	}
	if hasPlural {
		setCount++
	}

	// All set (3) → fully specified — handled above
	// None set (0) → kind-only declaration — enrichment candidate
	// Some set (1 or 2) → partial — error
	return setCount > 0 && setCount < 3
}
