package merger

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// Merger loads one or more Blueprint files, resolves their imports
// (files, URLs, Helm charts), merges all CRD entries, and exposes
// the result through Enabled(), All(), and Get().
//
// Entry point: one or more file paths from the CLI or constructRuntime.
// Everything else — source resolution, Helm rendering, deduplication —
// is internal to the merger.
//
// Merge rules:
//   - Sources within a Blueprint are loaded before spec.crds
//   - Inline spec.crds win on name conflict (local overrides remote)
//   - Duplicate names across independent Blueprint files are errors
//   - disabled CRDs are preserved and filtered by Enabled()
type Merger struct {
	// entryPoints are the initial file paths/URLs passed from the CLI (inrun or or generate)
	entryPoints []string

	// result holds the merged CRD entries after Merge() completes — keyed by CRD name
	result map[string]types.CRDEntry

	// security holds the security configuration of the final blueprint
	security types.BlueprintSecurity

	// gateway holds the gateway deployment config of the final blueprint
	gateway *types.GatewayConfig

	// publish holds the publishing and consumer policy of the final blueprint
	publish *types.PublishConfig

	// profiles holds the merged user-defined profile registry of the final blueprint
	profiles types.ProfileRegistry

	// specImports holds the spec.imports module list from the loaded Blueprint.
	// These are modules whose profiles and functions are merged at the Blueprint level.
	specImports []types.ModuleImport

	// functions holds the Blueprint-level user-defined function registry.
	functions types.KELFunctionRegistry

	// lifecycle holds the lifecycle policy of the final blueprint.
	lifecycle *types.BlueprintLifecycle

	// policy holds the platform policy block of the final blueprint.
	policy *types.BlueprintPolicy

	// merged tracks whether Merge() has been called
	merged bool

	// metadata gets the metadata from the document processed
	// used by cli and health endpoints
	apiMetadata apiMetadata

	// Refresh bypasses all local caches — git charts, remote Helm repos, and
	// remote file fetches are re-downloaded and the cached copies are overwritten.
	Refresh bool
}

type apiMetadata struct {
	APIVersion string              `json:"apiVersion" yaml:"apiVersion"`
	Kind       string              `json:"kind" yaml:"kind"`
	Metadata   types.BlueprintMeta `json:"metadata" yaml:"metadata"`
}

// New creates a Merger with the given entry point file paths or URLs.
// Accepts one or more paths — the same as passing --file multiple times or comma separated.
func New(paths ...string) *Merger {
	return &Merger{entryPoints: paths}
}

// Add appends additional entry point paths after construction.
// Returns the Merger for chaining.
func (m *Merger) Add(paths ...string) *Merger {
	m.entryPoints = append(m.entryPoints, paths...)
	return m
}

// Merge loads all entry points and their declared imports,
// resolves Helm charts, and produces a single deduplicated CRD map.
// Safe to call multiple times — re-merges on each call.
func (m *Merger) Merge() error {
	// seen lives here — top level only (tracks which file each name came from)
	seen := map[string]string{}
	merged := make(map[string]types.CRDEntry)

	for _, path := range m.entryPoints {
		// loadBlueprintFile manages its OWN internal dedup
		// it does NOT receive seen — only returns the final CRD map for this file
		crds, err := m.loadBlueprintFile(path)
		if err != nil {
			return fmt.Errorf("merger: loading %q: %w", path, err)
		}

		// Check duplicates HERE at the top level across entry points
		for name, crd := range crds {
			if err := checkDuplicate(seen, name, path); err != nil {
				return err
			}
			seen[name] = path
			merged[name] = crd
		}
	}

	m.result = merged
	m.merged = true

	logger.Debug().
		Int("entryPoints", len(m.entryPoints)).
		Int("total", len(m.result)).
		Int("enabled", len(m.Enabled())).
		Msg("merger: complete")
	return nil
}

// mergeCRDEntry merges an override entry on top of a base entry.
// The override wins on any field it explicitly declares.
// Fields that are zero/nil/empty in the override are inherited from the base.
//
// This is how Stack spec.crds works: you declare only what you want
// to change. Everything else — reconciler templates, validation rules,
// mutation rules, status config — is inherited from the source Blueprint.
func mergeCRDEntry(base, override types.CRDEntry) types.CRDEntry {
	result := base // start from the full base

	// ── Identity ─────────────────────────────────────────────────────────
	// Name always comes from the override (it's the merge key)
	// APITypes: override wins if declared, otherwise keep base
	if override.APITypes.Kind != "" {
		result.APITypes = override.APITypes
	}
	if override.CRDFile != "" {
		result.CRDFile = override.CRDFile
	}

	// ── Enabled ───────────────────────────────────────────────────────────
	// Only override if explicitly set to false — zero value (true) means
	// "not declared", so we keep the base value
	if !override.IsEnabled() {
		result.Enabled = override.Enabled
	}

	// ── Runtime tuning (operatorBox.reconcile) ──────────────────────────
	// Override only when non-zero — zero means "not declared in override"
	if override.Box().Reconcile != nil {
		if result.Box().Reconcile == nil {
			result.Box().Reconcile = &types.ReconcileConfig{}
		}
		ov := override.Box().Reconcile
		if ov.Workers > 0 {
			result.Box().Reconcile.Workers = ov.Workers
		}
		if ov.Resync.Duration != 0 {
			result.Box().Reconcile.Resync = ov.Resync
		}
		if ov.Queue.MaxDepth > 0 {
			result.Box().Reconcile.Queue.MaxDepth = ov.Queue.MaxDepth
		}
		if ov.Queue.FailureThreshold > 0 {
			result.Box().Reconcile.Queue.FailureThreshold = ov.Queue.FailureThreshold
		}
	}

	// ── Deletion Protection ───────────────────────────────────────────────
	if override.Box().Runtime != nil && override.Box().Runtime.DeletionProtection != nil {
		ovDp := override.Box().Runtime.DeletionProtection
		if result.Box().Runtime == nil {
			result.Box().Runtime = &types.RuntimeConfig{}
		}
		if result.Box().Runtime.DeletionProtection == nil {
			result.Box().Runtime.DeletionProtection = &types.DeletionProtectionOverride{}
		}
		dp := result.Box().Runtime.DeletionProtection
		if ovDp.ProtectCRD != nil {
			dp.ProtectCRD = ovDp.ProtectCRD
		}
		if ovDp.ProtectCRs != nil {
			dp.ProtectCRs = ovDp.ProtectCRs
		}
		if ovDp.StrictMode != nil {
			dp.StrictMode = ovDp.StrictMode
		}
	}

	// ── Namespace ─────────────────────────────────────────────────────────
	if override.Namespace != "" {
		result.Namespace = override.Namespace
	}

	// ── Dependencies ──────────────────────────────────────────────────────
	// Override replaces entirely if declared — partial dependency override
	// makes no semantic sense
	if len(override.DependsOn) > 0 {
		result.DependsOn = override.DependsOn
	}

	// ── Restricted namespaces — additive ──────────────────────────────────
	// Restrictions are additive: override adds to base, never removes
	if override.HasRestrictedNamespaces() {
		if result.Box().Runtime == nil {
			result.Box().Runtime = &types.RuntimeConfig{}
		}
		seen := map[string]struct{}{}
		for _, ns := range result.Box().Runtime.RestrictedNamespaces {
			seen[ns] = struct{}{}
		}
		for _, ns := range override.AllRestrictedNamespaces() {
			if _, ok := seen[ns]; !ok {
				result.Box().Runtime.RestrictedNamespaces = append(result.Box().Runtime.RestrictedNamespaces, ns)
			}
		}
	}

	// ── Allowed namespaces — additive ─────────────────────────────────────
	// Allowances are additive: override adds to base, never removes
	if override.HasAllowedNamespaces() {
		if result.Box().Runtime == nil {
			result.Box().Runtime = &types.RuntimeConfig{}
		}
		seen := map[string]struct{}{}
		for _, ns := range result.Box().Runtime.AllowedNamespaces {
			seen[ns] = struct{}{}
		}
		for _, ns := range override.AllAllowedNamespaces() {
			if _, ok := seen[ns]; !ok {
				result.Box().Runtime.AllowedNamespaces = append(result.Box().Runtime.AllowedNamespaces, ns)
			}
		}
	}

	// ── Finalizers — additive ─────────────────────────────────────────────
	if overrideFinals := override.Box().EffectiveFinalizers(); len(overrideFinals) > 0 {
		if result.Box().Runtime == nil {
			result.Box().Runtime = &types.RuntimeConfig{}
		}
		seen := map[string]struct{}{}
		for _, f := range result.Box().EffectiveFinalizers() {
			seen[f] = struct{}{}
		}
		names := result.Box().EffectiveFinalizers()
		for _, f := range overrideFinals {
			if _, ok := seen[f]; !ok {
				names = append(names, f)
			}
		}
		result.Box().Runtime.SetFinalizerNames(names)
	}

	// ── Reconcile config — override only declared blocks ──────────────────
	// If the override declares onCreate, it replaces the base onCreate.
	// If it doesn't declare it, the base onCreate is preserved.
	// Same for onReconcile, onDelete and status.
	if override.Box().EffectiveOnCreate() != nil ||
		override.Box().EffectiveOnReconcile() != nil ||
		override.Box().EffectiveOnDelete() != nil {
		if result.Box().Reconcile == nil {
			result.Box().Reconcile = &types.ReconcileConfig{}
		}
		rc := result.Box().Reconcile
		if override.Box().EffectiveOnCreate() != nil {
			rc.OnCreate = override.Box().EffectiveOnCreate()
		}
		if override.Box().EffectiveOnReconcile() != nil {
			rc.OnReconcile = override.Box().EffectiveOnReconcile()
		}
		if override.Box().EffectiveOnDelete() != nil {
			rc.OnDelete = override.Box().EffectiveOnDelete()
		}
	}

	// ── Status — override replaces if declared ────────────────────────────
	if override.Box().EffectiveStatus() != nil {
		if result.Box().Emit == nil {
			result.Box().Emit = &types.EmitConfig{}
		}
		result.Box().Emit.Status = override.Box().EffectiveStatus()
	}

	// ── Validation and mutation — override replaces if declared ───────────
	// Platform teams may want to add stricter rules in production via the
	// Stack. Replacing rather than merging is the safe behaviour —
	// merging rules from two imports could produce unexpected combinations.
	if v := override.EffectiveValidation(); v != nil {
		if result.Admission == nil {
			result.Admission = &types.AdmissionConfig{}
		}
		result.Admission.Validation = v
	}
	if m := override.EffectiveMutation(); m != nil {
		if result.Admission == nil {
			result.Admission = &types.AdmissionConfig{}
		}
		result.Admission.Mutation = m
	}

	// ── Conversion — override replaces if declared ────────────────────────
	if cv := override.EffectiveConversion(); cv != nil {
		if result.Admission == nil {
			result.Admission = &types.AdmissionConfig{}
		}
		result.Admission.Conversion = cv
	}

	// ── Endpoints ─────────────────────────────────────────────────────────
	if override.IsEnabledAllEndpoints() {
		result.Endpoints = override.Endpoints
	}

	return result
}

// ── Query methods ─────────────────────────────────────────────────────────────

// Enabled returns only CRD entries where enabled: true.
func (m *Merger) Enabled() map[string]types.CRDEntry {
	m.mustBeMerged()
	out := make(map[string]types.CRDEntry)
	for name, crd := range m.result {
		if crd.IsEnabled() {
			out[name] = crd
		}
	}
	return out
}

// All returns all CRD entries including disabled ones.
func (m *Merger) All() map[string]types.CRDEntry {
	m.mustBeMerged()
	return m.result
}

// Get returns a CRD entry by name. Returns (entry, true) if found.
func (m *Merger) Get(name string) (types.CRDEntry, bool) {
	m.mustBeMerged()
	crd, ok := m.result[name]
	return crd, ok
}

// Count returns total CRD count across all imports.
func (m *Merger) Count() int {
	m.mustBeMerged()
	return len(m.result)
}

// EnabledCount returns the number of enabled CRDs.
func (m *Merger) EnabledCount() int {
	return len(m.Enabled())
}

// ToSpec returns the merged result as a BlueprintSpec.
// Used by NewBlueprint and generate.Registry to consume the merged result.
func (m *Merger) ToSpec() types.BlueprintSpec {
	m.mustBeMerged()
	return types.BlueprintSpec{CRDs: m.result}
}

// ToSecurity returns the security config of the merged result as a BlueprintSecurity
// Used by NewBlueprint consume the merged result.
func (m *Merger) ToSecurity() types.BlueprintSecurity {
	m.mustBeMerged()
	return m.security
}

// ToGateway returns the gateway deployment config of the merged result.
// Used by BuildRuntimeBlueprint to populate Blueprint.Gateway.
func (m *Merger) ToGateway() *types.GatewayConfig {
	m.mustBeMerged()
	return m.gateway
}

// ToPublish returns the publish config of the merged result.
// Used by BuildRuntimeBlueprint to populate Blueprint.Publish.
func (m *Merger) ToPublish() *types.PublishConfig {
	m.mustBeMerged()
	return m.publish
}

// ToProfiles returns the merged user-defined profile registry of the merged result.
// Used by BuildRuntimeBlueprint to populate Blueprint.Profiles.
func (m *Merger) ToProfiles() types.ProfileRegistry {
	m.mustBeMerged()
	return m.profiles
}

// ToSpecImports returns the spec.imports module list from the loaded Blueprint.
// Used by BuildRuntimeBlueprint to populate Blueprint.Spec.Imports before profile expansion.
func (m *Merger) ToSpecImports() []types.ModuleImport {
	m.mustBeMerged()
	return m.specImports
}

// ToFunctions returns the Blueprint-level user-defined function registry.
// Used by BuildRuntimeBlueprint to populate Blueprint.Functions.
func (m *Merger) ToFunctions() types.KELFunctionRegistry {
	m.mustBeMerged()
	return m.functions
}

// ToLifecycle returns the lifecycle policy of the merged result.
// Used by BuildRuntimeBlueprint to populate Blueprint.lifecycle.
func (m *Merger) ToLifecycle() *types.BlueprintLifecycle {
	m.mustBeMerged()
	return m.lifecycle
}

// ToPolicy returns the platform policy block of the merged result.
func (m *Merger) ToPolicy() *types.BlueprintPolicy {
	m.mustBeMerged()
	return m.policy
}

// APIMetadata returns the merged result as a BlueprintMeta with apiversion and kind.
func (m *Merger) APIMetadata() apiMetadata {
	m.mustBeMerged()
	return m.apiMetadata
}

// FirstEntryDir returns the directory of the first entry point path.
// Used by the Blueprint parser to resolve relative crdFile paths.
func (m *Merger) FirstEntryDir() string {
	if len(m.entryPoints) == 0 {
		return ""
	}
	return filepath.Dir(m.entryPoints[0])
}

// ToUI returns a UI-friendly representation of the merged Blueprint.
// This method extracts only the fields needed for display in the Console:
//   - API version and kind (always "Blueprint" at runtime)
//   - Metadata (name, description, version, author, license)
//   - All merged CRD definitions
//
// Internal fields (Scheme, GroupVersionKind, etc.) are excluded because they
// have `yaml:"-" json:"-"` tags and won't be serialized to JSON.
//
// This method is used by the /blueprint/raw endpoint to provide a clean,
// readable view of the Blueprint that created the current operator.
func (m *Merger) ToUI() *types.BlueprintForUI {
	m.mustBeMerged()

	return &types.BlueprintForUI{
		APIVersion: m.apiMetadata.APIVersion,
		Kind:       config.BlueprintKind(),
		Metadata:   m.apiMetadata.Metadata,
		Spec: types.BlueprintSpecForUI{
			CRDs: m.result,
		},
	}
}
