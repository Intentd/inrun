package catalog

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// expandIncludes expands all include declarations at catalog and CRD level.
func (c *Catalog) expandIncludes() error {
	if c.Empty() {
		return fmt.Errorf("catalog is empty")
	}
	// ── Catalog Level ───────────────────────────────────────────────────────────────────

	if err := types.ExpandFunctionsInclude(&c.Functions, c.catalogDir); err != nil {
		return fmt.Errorf("functions: %w", err)
	}
	if err := types.ExpandProfileInclude(&c.Profiles, c.catalogDir); err != nil {
		return fmt.Errorf("profiles: %w", err)
	}
	if err := types.ExpandGatewayAPIAuthInclude(c.Gateway, c.catalogDir); err != nil {
		return fmt.Errorf("gateway API auth: %w", err)
	}
	if err := types.ExpandGatewayClustersInclude(c.Gateway, c.catalogDir); err != nil {
		return fmt.Errorf("gateway.clusters: %w", err)
	}

	// ── CRD Level ───────────────────────────────────────────────────────────────────

	for name, entry := range c.enabledCRDs {
		// Expand serve.include before enrichment so field hints are fully resolved.
		if err := populateAllServeFieldsFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}

		// Expand validation.include, mutation.include, conversion.include and status.include.
		if err := populateValidationRulesFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateMutationRulesFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateConversionPathsFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateStatusFieldsFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := types.PopulateExternalCallsFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateObserveInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateReconcilerFromInclude(&entry, c.catalogDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
	}
	return nil
}
