package types

// EnrichmentOutcome describes what happened when enrichment was attempted
// for a CRD entry. Used by inrun validate to print clear, actionable output.
type EnrichmentOutcome int

const (
	// EnrichmentNotNeeded — apiTypes is already fully specified
	EnrichmentNotNeeded EnrichmentOutcome = iota

	// EnrichmentApplied — kind-only declaration resolved to a built-in
	EnrichmentApplied

	// EnrichmentFailed — kind-only declaration did not match any built-in
	// and apiTypes is incomplete
	EnrichmentFailed
)
