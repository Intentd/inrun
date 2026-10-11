// pkg/merger/registry_stub.go
//
// Stand-in for registry.go in the runtime and gateway builds. See the
// comment at the top of registry.go for why.

//go:build runtime || gateway

package merger

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// loadRegistrySource is unavailable in this build — Registry Catalog
// imports are an authoring-time feature only. Pre-merge with the inrun CLI
// (inrun generate bundle) before deploying.
func (m *Merger) loadRegistrySource(src types.RegistrySource) (map[string]types.CRDEntry, error) {
	url, _ := src.ResolvedURL()
	return nil, fmt.Errorf("imports.registry (%q) is not supported in this build — "+
		"registry imports are authoring-time only; run 'inrun generate bundle' to "+
		"pre-merge before deploying the runtime or gateway", url)
}
