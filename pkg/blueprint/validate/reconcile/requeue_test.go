package reconcile

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blueprintWithRequeue(crdName string, rq *types.RequeueConfig) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{Requeue: rq},
		}},
	})
}

func TestValidateRequeue_NoConfig(t *testing.T) {
	e := blueprintWithRequeue("myapp", nil)
	assert.NoError(t, e.validateRequeue())
}

func TestValidateRequeue_EmptyAfter(t *testing.T) {
	e := blueprintWithRequeue("myapp", &types.RequeueConfig{After: ""})
	assert.NoError(t, e.validateRequeue())
}

func TestValidateRequeue_ValidDuration(t *testing.T) {
	for _, after := range []string{"30s", "5m", "1h", "500ms"} {
		e := blueprintWithRequeue("myapp", &types.RequeueConfig{After: after})
		assert.NoError(t, e.validateRequeue(), "after=%q", after)
	}
}

func TestValidateRequeue_ValidTemplate(t *testing.T) {
	e := blueprintWithRequeue("myapp", &types.RequeueConfig{
		After: `{{ .spec.checkInterval | default "60s" }}`,
	})
	assert.NoError(t, e.validateRequeue())
}

func TestValidateRequeue_InvalidAfter(t *testing.T) {
	e := blueprintWithRequeue("myapp", &types.RequeueConfig{After: "not-a-duration"})
	err := e.validateRequeue()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requeue.after")
	assert.Contains(t, err.Error(), "not-a-duration")
}

func TestValidateRequeue_InvalidAfter_Bare_Number(t *testing.T) {
	e := blueprintWithRequeue("myapp", &types.RequeueConfig{After: "60"})
	err := e.validateRequeue()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requeue.after")
}
