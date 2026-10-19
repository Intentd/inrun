// pkg/blueprint/parsek.go
package blueprint

import (
	"fmt"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/types"
)

// -----------------------------------------------------------------------------
//
//	YAML Builder
//
// -----------------------------------------------------------------------------

// BuildRuntimeBlueprint composes the runtime Blueprint for Inrun from merged configuration sources.
//
// This function is the central entry point for transforming the declarative Blueprint
// (YAML files + overrides) into a fully resolved, validated, and defaulted runtime
// representation (map of CRDEntry). It is called once at Inrun startup.
//
// The runtime Blueprint is used for:
//   - Reconciliation (controller loops, worker counts, resync periods)
//   - Admission webhooks (validation, mutation, deletion protection)
//   - Child resource materialisation (operatorBox onCreate/onReconcile)
//   - Status field resolution (Layer 2 status)
//   - Enrichment (owner, replicasets, pods, HPA)
//
// Processing steps (in order):
//
//  1. Extract merged spec, security, gateway, providers, and metadata from the merger.
//
//  2. Retrieve the list of enabled CRDs (the union of all declarations).
//
//  3. For each CRD entry:
//
//     - If a CRD file path is provided, populate APITypes from the actual CRD
//     (group, version, plural, scope) – this overrides any inline apiTypes.
//
//     - Run enrichment (EnrichCRDEntry) to resolve kind-only built-in declarations
//     (e.g., "Namespace" → group: "", version: "v1", plural: "namespaces").
//
//  4. Expand module imports declared in operatorBox blocks (module re-use across CRDs).
//
//  5. Initialize conversion and admission registries (for webhook generation).
//
//  6. Register validation, mutation, and conversion rules from each CRD entry.
//
//  7. Apply defaults (workers, max queue depth, resync, etc.) from the global config.
//
// Returns the final map of CRDEntry keyed by CRD name, ready for the reconciler and
// webhook servers.
//
// Errors are returned for:
//   - Missing or invalid CRD files
//   - Unknown built-in kinds during enrichment
//   - Partially specified apiTypes (e.g., kind+group but no version)
//   - Module import resolution failures
//   - Default assignment errors
//
// Example:
//
//	runtimeCRDs, err := blueprint.BuildRuntimeBlueprint(cfg, merger, "path/to/blueprint")
func (c *Blueprint) BuildRuntimeBlueprint(
	cfg *config.Config,
	m *merger.Merger,
	paths ...string,
) (map[string]types.CRDEntry, error) {

	c.Spec = m.ToSpec()
	c.Spec.Imports = m.ToSpecImports()
	c.Security = m.ToSecurity()
	c.Gateway = m.ToGateway()
	c.Publish = m.ToPublish()
	c.Profiles = m.ToProfiles()
	c.Functions = m.ToFunctions()
	c.enabledCRDs = m.Enabled()           // Enabled CRDs for all operations
	c.metadata = m.APIMetadata().Metadata // Metadata for CLI and health endpoints
	c.lifecycle = m.ToLifecycle()
	c.policy = m.ToPolicy()
	c.APIVersion = m.APIMetadata().APIVersion
	c.Kind = m.APIMetadata().Kind
	c.config = cfg
	c.blueprintDir = m.FirstEntryDir()

	// Expand all includes
	if err := c.expandIncludes(); err != nil {
		return nil, err
	}

	for name, entry := range c.enabledCRDs {
		// Populate APITypes from crdFile before enrichment so isFullySpecified sees
		// the correct values. crdFile is the source of truth — overwrites any apiTypes.
		// Clear CRDFile afterwards: the runtime (and any serialized bundle) must not
		// reference local filesystem paths that don't exist inside a container.
		if entry.CRDFile != "" {
			if err := PopulateAPITypesFromCRDFile(&entry, c.blueprintDir); err != nil {
				return nil, fmt.Errorf("CRD %q: %w", name, err)
			}
			entry.CRDFile = ""
		}

		// Enrich enabled CRDs
		outcome, err := EnrichCRDEntry(&entry)
		if err != nil {
			return nil, err
		}
		entry.EnrichmentOutcome = outcome
		c.enabledCRDs[name] = entry
	}

	// Expand spec.imports — merge profiles into the Blueprint-wide ProfileRegistry
	if err := c.expandBlueprintImports(); err != nil {
		return nil, err
	}

	// Expand Module imports declared in each operatorBox (resources, status, admission only)
	if err := c.expandModuleImports(); err != nil {
		return nil, err
	}

	// Apply any sythesized admission rules for:
	// required: true, default and override
	c.applyServeAdmissionSynthesis()

	// initialize conversion registry and admission registry
	c.conversionRegistry = NewInMemoryConversionRegistry()
	c.admissionRegistry = NewInMemoryAdmissionRegistry()

	// now safe to register rules
	for _, entry := range c.enabledCRDs {
		c.admissionRegistry.registerAdmissionRulesFromEntry(entry)
		c.conversionRegistry.registerConversionRulesFromSpec(entry)
	}

	// Apply defaults/GVK so CLI tools (simulate, validate, plan) get the same
	// field values as the runtime without needing to call Validate.
	if err := c.SetDefaults(cfg); err != nil {
		return nil, err
	}
	if err := c.SetGroupVersionKind(); err != nil {
		return nil, err
	}

	// Build all serve enabled CRDs
	c.serveEnabledCRDs = c.BuildServeEnabledCRDs()

	return c.enabledCRDs, nil
}
