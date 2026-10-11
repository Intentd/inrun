package observe

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithPreReconcile(pr types.PreReconcileConfig) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {
			APITypes: types.APITypes{
				Kind:    "Application",
				Version: "v1",
				Group:   "test.inrun.catalog",
			},
			OperatorBox: &types.OperatorBoxConfig{
				PreReconcile: &pr,
			},
		},
	})
}

func TestValidatePreReconcile_NoConfig(t *testing.T) {
	e := catalogWithPreReconcile(types.PreReconcileConfig{})
	assert.NoError(t, e.validatePreReconcile())
}

func TestValidatePreReconcile_Valid(t *testing.T) {
	e := catalogWithPreReconcile(types.PreReconcileConfig{
		ReconcileGate: &types.GateConditions{
			EventAware: true,
		},
	})
	assert.NoError(t, e.validatePreReconcile())
}

func TestValidatePreReconcile_Invalid(t *testing.T) {
	e := catalogWithPreReconcile(types.PreReconcileConfig{
		EnqueueGate: &types.GateConditions{
			EventAware: true,
		},
	})
	err := e.validatePreReconcile()
	require.Error(t, err)
	assert.ErrorContains(t, err, "'event aware' is only valid in reconcileGate")
}
