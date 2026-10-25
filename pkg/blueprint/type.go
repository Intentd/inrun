package blueprint

import (
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
)

// -----------------------------------------------------------------------------
// Structs
// -----------------------------------------------------------------------------
type Blueprint struct {
	APIVersion string                    `yaml:"apiVersion"`
	Kind       string                    `yaml:"kind"`
	Spec       types.BlueprintSpec       `yaml:"spec"`
	Security   types.BlueprintSecurity   `yaml:"security"`
	Functions  types.KELFunctionRegistry `yaml:"functions,omitempty"`
	Presets    types.PresetRegistry      `yaml:"presets,omitempty"`
	Gateway    *types.GatewayConfig      `yaml:"gateway,omitempty"`
	Publish    *types.PublishConfig      `yaml:"publish,omitempty"`

	StackMetadata types.BlueprintMeta `yaml:"metadata"`

	// Internal — enabledCRDs is enriched and validated; Spec.CRDs holds all (including disabled)
	metadata           types.BlueprintMeta       `yaml:"-" json:"-"`
	lifecycle          *types.BlueprintLifecycle `yaml:"-" json:"-"`
	policy             *types.BlueprintPolicy    `yaml:"-" json:"-"`
	enabledCRDs        map[string]types.CRDEntry `yaml:"-" json:"-"`
	serveEnabledCRDs   []*types.CRDEntry         `yaml:"-" json:"-"`
	blueprintDir       string                    `yaml:"-" json:"-"`
	conversionRegistry *InMemoryConversionRegistry
	admissionRegistry  *InMemoryAdmissionRegistry

	// config for managing blueprint-related user inputs from env
	config *config.Config

	// Warnings collects non‑fatal validation messages for this CRD.
	Warnings types.Warnings `json:"-"` // not serialized
	// Info collects additional validation information for this CRD
	Info types.Info `json:"-"` // not serialized

	// Indexes for O(1) lookups
	kindIndex       map[string]string `yaml:"-" json:"-"` // kind -> crd name
	nameIndex       map[string]string `yaml:"-" json:"-"` // lowercase(map key) -> crd name
	pluralIndex     map[string]string `yaml:"-" json:"-"` // plural resource name -> crd name
	apiVersionIndex map[string]string `yaml:"-" json:"-"` // apiVersion -> crd name
	gvkIndex        map[string]string `yaml:"-" json:"-"` // gvk.String() -> crd name
	gvrIndex        map[string]string `yaml:"-" json:"-"` // gvr.String() -> crd name
	targetIndex     map[string]string `yaml:"-" json:"-"` // target -> crd name

}

// GatewayClusters returns the gateway.clusters entries map, or nil when none are declared.
func (c *Blueprint) GatewayClusters() map[string]types.GatewayClusterConfig {
	if c == nil || c.Gateway == nil || c.Gateway.Clusters == nil {
		return nil
	}
	return c.Gateway.Clusters.Entries
}

// GatewayClusterCount returns the number of gateway clusters defined.
func (c *Blueprint) GatewayClusterCount() int {
	return len(c.GatewayClusters())
}

// GatewayClustersEmpty returns true if no gateway clusters are defined.
func (c *Blueprint) GatewayClustersEmpty() bool {
	return c.GatewayClusterCount() == 0
}

// EnabledCRDs returns a map of enabled CRDs keyed by their name.
func (c *Blueprint) EnabledCRDs() map[string]types.CRDEntry {
	return c.enabledCRDs
}

// ListEnabledCRDs returns a slice of all enabled CRD entries.
// Use this for iteration when the map key is not needed.
func (c *Blueprint) ListEnabledCRDs() []types.CRDEntry {
	entries := make([]types.CRDEntry, 0, c.Len())
	for _, crd := range c.enabledCRDs {
		entries = append(entries, crd)
	}
	return entries
}

// ListEnabledCRDPointers returns a slice of pointers to all enabled CRD entries.
// Use this when you need to modify CRD entries or avoid copying.
func (c *Blueprint) ListEnabledCRDPointers() []*types.CRDEntry {
	entries := make([]*types.CRDEntry, 0, c.Len())
	for _, crd := range c.enabledCRDs {
		crdCopy := crd
		entries = append(entries, &crdCopy)
	}
	return entries
}

// EnabledCRDsList returns a slice of all enabled CRD entries.
func (c *Blueprint) EnabledCRDsList() []types.CRDEntry {
	entries := make([]types.CRDEntry, 0, c.Len())
	for _, crd := range c.enabledCRDs {
		entries = append(entries, crd)
	}
	return entries
}

// EnabledCRDMap returns the raw map of enabled CRDs.
func (c *Blueprint) EnabledCRDMap() map[string]types.CRDEntry {
	return c.enabledCRDs
}

// AllCRDs returns all CRDs including disabled ones (from Spec).
func (c *Blueprint) AllCRDs() map[string]types.CRDEntry {
	return c.Spec.CRDs
}

// UserNotes returns all user defined notes in the blueprint
func (c *Blueprint) UserFunctions() types.KELFunctionRegistry {
	if c == nil {
		return types.KELFunctionRegistry{}
	}
	return c.Functions
}

// UserPresets returns all user defined presets in the blueprint
func (c *Blueprint) UserPresets() *types.PresetRegistry {
	if c == nil {
		return nil
	}
	return &c.Presets
}

// Empty reports true when the blueprint is nil.
func (c *Blueprint) Empty() bool {
	return c == nil
}

// EnabledCRDsEmpty reports true when enabled crds is 0.
func (c *Blueprint) EnabledCRDsEmpty() bool {
	return len(c.enabledCRDs) == 0
}

// Len returns the number of enabled CRDs in this blueprint
func (c *Blueprint) Len() int {
	return len(c.enabledCRDs)
}

// SetConfig sets the config on the Blueprint. Called by pipeline.NewBlueprint
// before BuildRuntimeBlueprint so that config-dependent methods work correctly.
func (c *Blueprint) SetConfig(cfg *config.Config) {
	c.config = cfg
}

// HasIntentFiles reports whether intent.yaml or intent.json are present in the
// blueprint directory. Used at validate time to enforce publish.tests.intent: true.
func (c *Blueprint) HasIntentFiles() bool {
	if c.blueprintDir == "" {
		return false
	}
	for _, name := range []string{"intent.yaml", "intent.json"} {
		if _, err := os.Stat(filepath.Join(c.blueprintDir, name)); err == nil {
			return true
		}
	}
	return false
}

// Metadata returns the Blueprint metadata.
func (c *Blueprint) Metadata() types.BlueprintMeta {
	return c.metadata
}

// Lifecycle returns the lifecycle policy block, or nil if absent.
func (c *Blueprint) Lifecycle() *types.BlueprintLifecycle {
	return c.lifecycle
}

// Policy returns the platform policy block, or nil if absent.
func (c *Blueprint) Policy() *types.BlueprintPolicy {
	return c.policy
}

// Deprecation returns the raw deprecation block, or nil if absent.
func (c *Blueprint) Deprecation() *types.BlueprintDeprecation {
	if c.lifecycle == nil {
		return nil
	}
	return c.lifecycle.Deprecation
}

// IsDeprecated returns true if the Blueprint is deprecated.
func (c *Blueprint) IsDeprecated() bool {
	if c.lifecycle == nil {
		return false
	}
	return c.lifecycle.IsDeprecated()
}

// IsMigrated returns true if the Blueprint is deprecated and has a migration target.
func (c *Blueprint) IsMigrated() bool {
	if c.lifecycle == nil || c.lifecycle.Deprecation == nil {
		return false
	}
	return c.lifecycle.Deprecation.MigrationTarget() != ""
}

// MigrationTarget returns the value of the MigratedTo field.
// If the lifecycle or deprecation block is nil, it returns an empty string.
func (c *Blueprint) MigrationTarget() string {
	if c.lifecycle == nil || c.lifecycle.Deprecation == nil {
		return ""
	}
	return c.lifecycle.Deprecation.MigrationTarget()
}

// MigrationMessage returns the deprecation message.
func (c *Blueprint) MigrationMessage() string {
	if c.lifecycle == nil || c.lifecycle.Deprecation == nil {
		return ""
	}
	return c.lifecycle.Deprecation.MigrationMessage()
}

// CRDEntry returns the enabled CRD entry for the given name.
func (c *Blueprint) CRDEntry(name string) (types.CRDEntry, bool) {
	entry, ok := c.enabledCRDs[name]
	return entry, ok
}

// Scheme builds and returns a runtime.Scheme with all Blueprint types registered.
func (c *Blueprint) Scheme() (*runtime.Scheme, error) {
	if c == nil {
		return nil, nil
	}
	return NewSchemeRegistry(c)
}

// Empty blueprint for testing
func NewEmptyBlueprint() *Blueprint {
	return &Blueprint{}
}

// Dir returns the directory containing the blueprint file, used to resolve
// relative crdFile paths. Returns "." when no directory is recorded.
func (c *Blueprint) Dir() string {
	if c.blueprintDir == "" {
		return "."
	}
	return c.blueprintDir
}
