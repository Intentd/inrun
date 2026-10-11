package validate

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
)

// newExec wraps a *catalog.Catalog in an executor for use in white-box tests.
func newExec(k *catalog.Catalog) *executor {
	return &executor{k: k}
}

// newCatalogExec builds an executor with pre-set CRDs for tests.
func newCatalogExec(crds map[string]types.CRDEntry) *executor {
	return newExec(catalog.NewCatalogForTest(crds))
}

// crd returns a test CRDEntry by name.
func (e *executor) crd(name string) *types.CRDEntry {
	crd, ok := e.k.Enabled()[name]
	if !ok {
		return nil
	}
	return &crd
}

// serveEntry builds a minimal serve-enabled CRDEntry for tests.
func serveEntry(kind, targetName string) types.CRDEntry {
	tv := types.ServeTargetValue{}
	if targetName != "" {
		tv.Entries = map[string]*types.ServeTargetConfig{
			targetName: {Primary: true},
		}
	}
	return types.CRDEntry{
		APITypes: types.APITypes{Kind: kind},
		Serve: &types.ServeConfig{
			Enabled: true,
			Target:  tv,
		},
	}
}

// withAliases merges alias entries into a CRDEntry's ServeTarget.
func withAliases(e types.CRDEntry, aliases map[string]*types.ServeTargetConfig) types.CRDEntry {
	if e.Serve.Target.Entries == nil {
		e.Serve.Target.Entries = make(map[string]*types.ServeTargetConfig)
	}
	for name, cfg := range aliases {
		e.Serve.Target.Entries[name] = cfg
	}
	return e
}
