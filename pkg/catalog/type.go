package catalog

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
)

// -----------------------------------------------------------------------------
// Variables
// -----------------------------------------------------------------------------
var (
	// For updating the CRD instance - needed for lookups
	resourceTypeMap = map[reflect.Type]string{}
)

// -----------------------------------------------------------------------------
// Structs
// -----------------------------------------------------------------------------
type Catalog struct {
	APIVersion string                `yaml:"apiVersion"`
	Kind       string                `yaml:"kind"`
	Spec       types.CatalogSpec     `yaml:"spec"`
	Security   types.CatalogSecurity `yaml:"security"`
	Notes      types.NoteRegistry    `yaml:"notes,omitempty"`
	Profiles   types.ProfileRegistry `yaml:"profiles,omitempty"`
	Gateway    *types.GatewayConfig  `yaml:"gateway,omitempty"`
	Publish    *types.PublishConfig  `yaml:"publish,omitempty"`

	StackMetadata types.CatalogMeta `yaml:"metadata"`

	// Internal — enabledCRDs is enriched and validated; Spec.CRDs holds all (including disabled)
	metadata           types.CatalogMeta         `yaml:"-" json:"-"`
	lifecycle          *types.CatalogLifecycle   `yaml:"-" json:"-"`
	policy             *types.CatalogPolicy      `yaml:"-" json:"-"`
	enabledCRDs        map[string]types.CRDEntry `yaml:"-" json:"-"`
	serveEnabledCRDs   []*types.CRDEntry         `yaml:"-" json:"-"`
	catalogDir         string                    `yaml:"-" json:"-"`
	conversionRegistry *InMemoryConversionRegistry
	admissionRegistry  *InMemoryAdmissionRegistry

	// config for managing catalog-related user inputs from env
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
func (k *Catalog) GatewayClusters() map[string]types.GatewayClusterConfig {
	if k == nil || k.Gateway == nil || k.Gateway.Clusters == nil {
		return nil
	}
	return k.Gateway.Clusters.Entries
}

// GatewayClusterCount returns the number of gateway clusters defined.
func (k *Catalog) GatewayClusterCount() int {
	return len(k.GatewayClusters())
}

// GatewayClustersEmpty returns true if no gateway clusters are defined.
func (k *Catalog) GatewayClustersEmpty() bool {
	return k.GatewayClusterCount() == 0
}

// EnabledCRDs returns a map of enabled CRDs keyed by their name.
func (k *Catalog) EnabledCRDs() map[string]types.CRDEntry {
	return k.enabledCRDs
}

// ListEnabledCRDs returns a slice of all enabled CRD entries.
// Use this for iteration when the map key is not needed.
func (k *Catalog) ListEnabledCRDs() []types.CRDEntry {
	entries := make([]types.CRDEntry, 0, k.Len())
	for _, crd := range k.enabledCRDs {
		entries = append(entries, crd)
	}
	return entries
}

// ListEnabledCRDPointers returns a slice of pointers to all enabled CRD entries.
// Use this when you need to modify CRD entries or avoid copying.
func (k *Catalog) ListEnabledCRDPointers() []*types.CRDEntry {
	entries := make([]*types.CRDEntry, 0, k.Len())
	for _, crd := range k.enabledCRDs {
		crdCopy := crd
		entries = append(entries, &crdCopy)
	}
	return entries
}

// EnabledCRDsList returns a slice of all enabled CRD entries.
func (k *Catalog) EnabledCRDsList() []types.CRDEntry {
	entries := make([]types.CRDEntry, 0, k.Len())
	for _, crd := range k.enabledCRDs {
		entries = append(entries, crd)
	}
	return entries
}

// EnabledCRDMap returns the raw map of enabled CRDs.
func (k *Catalog) EnabledCRDMap() map[string]types.CRDEntry {
	return k.enabledCRDs
}

// AllCRDs returns all CRDs including disabled ones (from Spec).
func (k *Catalog) AllCRDs() map[string]types.CRDEntry {
	return k.Spec.CRDs
}

// UserNotes returns all user defined notes in the catalog
func (k *Catalog) UserNotes() types.NoteRegistry {
	if k == nil {
		return types.NoteRegistry{}
	}
	return k.Notes
}

// UserProfiles returns all user defined profiles in the catalog
func (k *Catalog) UserProfiles() *types.ProfileRegistry {
	if k == nil {
		return nil
	}
	return &k.Profiles
}

// Empty reports true when the catalog is nil.
func (k *Catalog) Empty() bool {
	return k == nil
}

// EnabledCRDsEmpty reports true when enabled crds is 0.
func (k *Catalog) EnabledCRDsEmpty() bool {
	return len(k.enabledCRDs) == 0
}

// Len returns the number of enabled CRDs in this catalog
func (k *Catalog) Len() int {
	return len(k.enabledCRDs)
}

// SetConfig sets the config on the Catalog. Called by pipeline.NewCatalog
// before BuildRuntimeCatalog so that config-dependent methods work correctly.
func (k *Catalog) SetConfig(kfg *config.Config) {
	k.config = kfg
}

// HasIntentFiles reports whether intent.yaml or intent.json are present in the
// catalog directory. Used at validate time to enforce publish.tests.intent: true.
func (k *Catalog) HasIntentFiles() bool {
	if k.catalogDir == "" {
		return false
	}
	for _, name := range []string{"intent.yaml", "intent.json"} {
		if _, err := os.Stat(filepath.Join(k.catalogDir, name)); err == nil {
			return true
		}
	}
	return false
}

// Metadata returns the Catalog metadata.
func (k *Catalog) Metadata() types.CatalogMeta {
	return k.metadata
}

// Lifecycle returns the lifecycle policy block, or nil if absent.
func (k *Catalog) Lifecycle() *types.CatalogLifecycle {
	return k.lifecycle
}

// Policy returns the platform policy block, or nil if absent.
func (k *Catalog) Policy() *types.CatalogPolicy {
	return k.policy
}

// Deprecation returns the raw deprecation block, or nil if absent.
func (k *Catalog) Deprecation() *types.CatalogDeprecation {
	if k.lifecycle == nil {
		return nil
	}
	return k.lifecycle.Deprecation
}

// IsDeprecated returns true if the Catalog is deprecated.
func (k *Catalog) IsDeprecated() bool {
	if k.lifecycle == nil {
		return false
	}
	return k.lifecycle.IsDeprecated()
}

// IsMigrated returns true if the Catalog is deprecated and has a migration target.
func (k *Catalog) IsMigrated() bool {
	if k.lifecycle == nil || k.lifecycle.Deprecation == nil {
		return false
	}
	return k.lifecycle.Deprecation.MigrationTarget() != ""
}

// MigrationTarget returns the value of the MigratedTo field.
// If the lifecycle or deprecation block is nil, it returns an empty string.
func (k *Catalog) MigrationTarget() string {
	if k.lifecycle == nil || k.lifecycle.Deprecation == nil {
		return ""
	}
	return k.lifecycle.Deprecation.MigrationTarget()
}

// MigrationMessage returns the deprecation message.
func (k *Catalog) MigrationMessage() string {
	if k.lifecycle == nil || k.lifecycle.Deprecation == nil {
		return ""
	}
	return k.lifecycle.Deprecation.MigrationMessage()
}

// CRDEntry returns the enabled CRD entry for the given name.
func (k *Catalog) CRDEntry(name string) (types.CRDEntry, bool) {
	entry, ok := k.enabledCRDs[name]
	return entry, ok
}

// Scheme builds and returns a runtime.Scheme with all Catalog types registered.
func (k *Catalog) Scheme() (*runtime.Scheme, error) {
	if k == nil {
		return nil, nil
	}
	return NewSchemeRegistry(k)
}

// Empty catalog for testing
func NewEmptyCatalog() *Catalog {
	return &Catalog{}
}
