// pkg/catalog/serialize.go
package catalog

import (
	"fmt"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// SerializeExpanded serializes the post-BuildRuntimeCatalog state to YAML.
//
// The output is a valid Catalog YAML with fully expanded CRDs (all module
// imports already inlined, imports: field absent). This is what the bundle's
// ConfigMap embeds — the runtime can load it without any OCI pulls.
//
// Must be called after BuildRuntimeCatalog; returns an error if no CRDs
// are present (indicates expansion hasn't been run yet).
func (c *Catalog) SerializeExpanded() ([]byte, error) {
	if c.EnabledCRDsEmpty() && !c.IsStandaloneGateway() {
		return nil, fmt.Errorf("Catalog has no enabled CRDs")
	}

	// Reconstruct a CatalogFile from the fully expanded state.
	// Always kind: Catalog — the bundle has no imports; Stack sources become
	// plain Catalogs after expansion so the runtime always takes the loadCatalog path.
	kf := types.CatalogFile{
		APIVersion: c.APIVersion,
		Kind:       config.CatalogKind(),
		Metadata:   c.metadata,
		Spec:       types.CatalogSpec{CRDs: withoutLocalPaths(c.enabledCRDs)},
		Security:   c.Security,
		Gateway:    c.Gateway,
		Profiles:   c.Profiles,
		Functions:  c.Functions,
	}

	out, err := yaml.Marshal(kf)
	if err != nil {
		return nil, fmt.Errorf("serializing expanded catalog: %w", err)
	}
	return out, nil
}

// withoutLocalPaths returns the CRDs with the development-only file fields
// (crdFile, crFiles, setup) cleared: the serialized Catalog runs in a pod,
// where those local paths don't exist.
func withoutLocalPaths(crds map[string]types.CRDEntry) map[string]types.CRDEntry {
	out := make(map[string]types.CRDEntry, len(crds))
	for name, crd := range crds {
		crd.CRDFile = ""
		crd.CRFiles = nil
		crd.Setup = nil
		out[name] = crd
	}
	return out
}
