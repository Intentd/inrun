package blueprint

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// expandIncludes expands all include declarations at blueprint and CRD level.
func (c *Blueprint) expandIncludes() error {
	if c.Empty() {
		return fmt.Errorf("blueprint is empty")
	}
	// ── Blueprint Level ───────────────────────────────────────────────────────────────────

	if err := types.ExpandFunctionsInclude(&c.Functions, c.blueprintDir); err != nil {
		return fmt.Errorf("functions: %w", err)
	}
	if err := types.ExpandPresetInclude(&c.Presets, c.blueprintDir); err != nil {
		return fmt.Errorf("presets: %w", err)
	}
	if err := types.ExpandGatewayAPIAuthInclude(c.Gateway, c.blueprintDir); err != nil {
		return fmt.Errorf("gateway API auth: %w", err)
	}
	if err := types.ExpandGatewayClustersInclude(c.Gateway, c.blueprintDir); err != nil {
		return fmt.Errorf("gateway.clusters: %w", err)
	}

	// ── CRD Level ───────────────────────────────────────────────────────────────────

	for name, entry := range c.enabledCRDs {
		// Expand serve.include before enrichment so field hints are fully resolved.
		if err := populateAllServeFieldsFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}

		// Expand validation.include, mutation.include, conversion.include and status.include.
		if err := populateValidationRulesFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateMutationRulesFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateConversionPathsFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateStatusFieldsFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := types.PopulateExternalCallsFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateObserveInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		if err := populateReconcilerFromInclude(&entry, c.blueprintDir); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
	}
	return nil
}
