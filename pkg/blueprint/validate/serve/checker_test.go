package serve

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/types"
)

// newExec returns a checker over bp.
func newExec(bp *blueprint.Blueprint) *checker { return &checker{check.New(bp, nil)} }

// newBlueprintExec returns a checker over a test Blueprint holding crds.
func newBlueprintExec(crds map[string]types.CRDEntry) *checker {
	return newExec(blueprint.NewBlueprintForTest(crds))
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
