package migrate

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/types"
)

// newExec returns a checker over bp.
func newExec(bp *blueprint.Blueprint) *checker { return &checker{check.New(bp, nil)} }

// crdPtr returns a pointer to the named CRD entry from the executor's blueprint.
func crdPtr(e *checker, name string) *types.CRDEntry {
	entry := e.Blueprint.EnabledCRDs()[name]
	return &entry
}

// newBlueprintExec returns a checker over a test Blueprint holding crds.
func newBlueprintExec(crds map[string]types.CRDEntry) *checker {
	return newExec(blueprint.NewBlueprintForTest(crds))
}

func blueprintWithMigrate(migrate *types.CRMigrate) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		"app": {Migrate: migrate},
	})
}

func blueprintWithMigrateAndField(migrate *types.CRMigrate, serve *types.ServeConfig) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		"app": {Migrate: migrate, Serve: serve},
	})
}
