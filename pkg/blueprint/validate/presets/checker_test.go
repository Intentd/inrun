package presets

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
