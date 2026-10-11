package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithRequeue(crdName string, rq *types.RequeueConfig) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{Requeue: rq},
		}},
	})
}

func TestValidateRequeue_NoConfig(t *testing.T) {
	k := catalogWithRequeue("myapp", nil)
	assert.NoError(t, k.validateRequeue())
}

func TestValidateRequeue_EmptyAfter(t *testing.T) {
	k := catalogWithRequeue("myapp", &types.RequeueConfig{After: ""})
	assert.NoError(t, k.validateRequeue())
}

func TestValidateRequeue_ValidDuration(t *testing.T) {
	for _, after := range []string{"30s", "5m", "1h", "500ms"} {
		k := catalogWithRequeue("myapp", &types.RequeueConfig{After: after})
		assert.NoError(t, k.validateRequeue(), "after=%q", after)
	}
}

func TestValidateRequeue_ValidTemplate(t *testing.T) {
	k := catalogWithRequeue("myapp", &types.RequeueConfig{
		After: `{{ .spec.checkInterval | default "60s" }}`,
	})
	assert.NoError(t, k.validateRequeue())
}

func TestValidateRequeue_InvalidAfter(t *testing.T) {
	k := catalogWithRequeue("myapp", &types.RequeueConfig{After: "not-a-duration"})
	err := k.validateRequeue()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requeue.after")
	assert.Contains(t, err.Error(), "not-a-duration")
}

func TestValidateRequeue_InvalidAfter_Bare_Number(t *testing.T) {
	k := catalogWithRequeue("myapp", &types.RequeueConfig{After: "60"})
	err := k.validateRequeue()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requeue.after")
}
