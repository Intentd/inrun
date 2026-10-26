package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
)

func finalizerTimeoutExec(crdTimeout, targetTimeout string) *checker {
	crd := types.CRDEntry{
		APITypes: types.APITypes{Group: "demo.inrun.dev", Version: "v1", Kind: "App", Plural: "apps"},
		OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{
			Finalizers: &types.FinalizerConfig{Timeout: crdTimeout},
		}},
	}
	if targetTimeout != "" {
		crd.Serve = &types.ServeConfig{Target: types.ServeTargetValue{Entries: map[string]*types.ServeTargetConfig{
			"web": {OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{
				Finalizers: &types.FinalizerConfig{Timeout: targetTimeout},
			}}},
		}}}
	}
	return newExec(blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"app": crd}))
}

func TestValidateFinalizerTimeout(t *testing.T) {
	for _, ok := range []string{"", "30m", "2h", "1d"} {
		assert.NoError(t, finalizerTimeoutExec(ok, "").validateFinalizerTimeout(), "timeout=%q", ok)
	}
	for _, bad := range []string{"soon", "0s", "-5m"} {
		err := finalizerTimeoutExec(bad, "").validateFinalizerTimeout()
		assert.ErrorContains(t, err, "runtime.finalizers.timeout", "timeout=%q", bad)
	}
	err := finalizerTimeoutExec("", "later").validateFinalizerTimeout()
	assert.ErrorContains(t, err, "serve.target[web].runtime.finalizers.timeout")
}
