package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intentd/inrun/pkg/types"
)

func TestSimulateSpec_ValidateSetup(t *testing.T) {
	apply := &types.SimulateSpec{Prepare: &types.Prepare{Apply: []types.ProvisionApplyEntry{{Path: "./ns.yaml"}}}}
	assert.Empty(t, apply.ValidateSetup(), "manifests are what simulate applies")
	assert.Equal(t, []string{"./ns.yaml"}, apply.PreparePaths())
	assert.Empty(t, (&types.SimulateSpec{}).ValidateSetup())

	beyond := &types.SimulateSpec{Prepare: &types.Prepare{
		Apply: []types.ProvisionApplyEntry{{Path: "./ns.yaml", Wait: []types.ProvisionWait{{Kind: "Namespace", Name: "a"}}}},
		Helm:  []types.ProvisionHelmInstall{{}},
		Wait:  []types.ProvisionWait{{Kind: "Namespace", Name: "a"}},
	}}
	errs := beyond.ValidateSetup()
	assert.Len(t, errs, 3)
	assert.Contains(t, errs[0], "spec.prepare.helm")
	assert.Contains(t, errs[1], "spec.prepare.wait")
	assert.Contains(t, errs[2], "spec.prepare.apply[0].wait")
}
