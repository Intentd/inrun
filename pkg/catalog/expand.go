package catalog

import (
	"fmt"
	"github.com/intentd/inrun/pkg/types"
)

// expandIncludes expands all include declarations at catalog and CRD level.
func (k *Catalog) expandIncludes() error {
	if k.Empty() {
		return fmt.Errorf("catalog is empty")
	}
	// ── Catalog Level ───────────────────────────────────────────────────────────────────

	if err := types.ExpandNotesInclude(&k.Notes, k.catalogDir); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	if err := types.ExpandProfileInclude(&k.Profiles, k.catalogDir); err != nil {
		return fmt.Errorf("profiles: %w", err)
	}
	if err := types.ExpandGatewayAPIAuthInclude(k.Gateway, k.catalogDir); err != nil {
		return fmt.Errorf("gateway API auth: %w", err)
	}
	if err := types.ExpandGatewayClustersInclude(k.Gateway, k.catalogDir); err != nil {
		return fmt.Errorf("gateway.clusters: %w", err)
	}

	// ── CRD Level ───────────────────────────────────────────────────────────────────

	for name, entry := range k.enabledCRDs {
		// Expand serve.include before enrichment so field hints are fully resolved.
		if err := populateAllServeFieldsFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}

		// Expand validation.include, mutation.include, conversion.include and status.include.
		if err := populateValidationRulesFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateMutationRulesFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateConversionPathsFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateStatusFieldsFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := types.PopulateExternalCallsFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateObserveInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateReconcilerFromInclude(&entry, k.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
	}
	return nil
}
