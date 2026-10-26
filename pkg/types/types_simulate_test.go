package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intentd/inrun/pkg/types"
)

func TestSimulateSpec_ValidateSetup(t *testing.T) {
	apply := &types.SimulateSpec{Setup: &types.SetupConfig{Apply: []types.SetupApplyEntry{{Path: "./ns.yaml"}}}}
	assert.Empty(t, apply.ValidateSetup(), "manifests are what simulate applies")
	assert.Equal(t, []string{"./ns.yaml"}, apply.SetupPaths())
	assert.Empty(t, (&types.SimulateSpec{}).ValidateSetup())

	beyond := &types.SimulateSpec{Setup: &types.SetupConfig{
		Apply: []types.SetupApplyEntry{{Path: "./ns.yaml", Wait: []types.SetupWait{{Kind: "Namespace", Name: "a"}}}},
		Helm:  []types.SetupHelmInstall{{}},
		Wait:  []types.SetupWait{{Kind: "Namespace", Name: "a"}},
	}}
	errs := beyond.ValidateSetup()
	assert.Len(t, errs, 3)
	assert.Contains(t, errs[0], "spec.setup.helm")
	assert.Contains(t, errs[1], "spec.setup.wait")
	assert.Contains(t, errs[2], "spec.setup.apply[0].wait")
}
