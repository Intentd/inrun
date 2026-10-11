package serve

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/types"
)

// newExec returns a checker over cat.
func newExec(cat *catalog.Catalog) *checker { return &checker{check.New(cat, nil)} }

// newCatalogExec returns a checker over a test Catalog holding crds.
func newCatalogExec(crds map[string]types.CRDEntry) *checker {
	return newExec(catalog.NewCatalogForTest(crds))
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
