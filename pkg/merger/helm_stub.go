// pkg/merger/helm_stub.go
//
// Stand-in for helm.go in the runtime and gateway builds, which exclude the
// Helm SDK entirely (see the comment at the top of helm.go for why). Keeps
// file.go's call site compiling in all three build configurations; a
// production Blueprint with an imports.helm: entry gets a clear error instead
// of a missing-symbol build failure.

//go:build runtime || gateway

package merger

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// loadHelmSource is unavailable in this build — Helm-chart Blueprint imports
// are an authoring-time feature only. Pre-merge with the inrun CLI
// (inrun generate bundle) before deploying.
func (m *Merger) loadHelmSource(src types.HelmSource) (map[string]types.CRDEntry, error) {
	return nil, fmt.Errorf("imports.helm (chart %q) is not supported in this build — "+
		"Helm-chart imports are authoring-time only; run 'inrun generate bundle' to "+
		"pre-merge before deploying the runtime or gateway", src.Chart)
}
