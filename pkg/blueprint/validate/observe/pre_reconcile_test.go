package observe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithPreReconcile(pr types.PreReconcileConfig) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			APITypes: types.APITypes{
				Kind:    "Application",
				Version: "v1",
				Group:   "test.inrun.blueprint",
			},
			OperatorBox: &types.OperatorBoxConfig{
				PreReconcile: &pr,
			},
		},
	})
}

func TestValidatePreReconcile_NoConfig(t *testing.T) {
	e := blueprintWithPreReconcile(types.PreReconcileConfig{})
	assert.NoError(t, e.validatePreReconcile())
}

func TestValidatePreReconcile_Valid(t *testing.T) {
	e := blueprintWithPreReconcile(types.PreReconcileConfig{
		ReconcileGate: &types.GateConditions{
			EventAware: true,
		},
	})
	assert.NoError(t, e.validatePreReconcile())
}

func TestValidatePreReconcile_Invalid(t *testing.T) {
	e := blueprintWithPreReconcile(types.PreReconcileConfig{
		EnqueueGate: &types.GateConditions{
			EventAware: true,
		},
	})
	err := e.validatePreReconcile()
	require.Error(t, err)
	assert.ErrorContains(t, err, "'event aware' is only valid in reconcileGate")
}
