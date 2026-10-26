package reconcile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func reconcileWithConditions(when []types.Condition, notes ...types.UserDefinedFunction) error {
	rc := &types.ReconcileConfig{
		Args: map[string]interface{}{
			"quotaGB": "{{ .spec.quotaGB }}",
			"plan":    map[string]interface{}{"tier": "{{ .external.plan.json.tier }}"},
		},
		External: []types.ExternalCallSpec{{CallSpec: types.CallSpec{URL: "http://plans/{{ .metadata.name }}"}, Name: "plan"}},
		When:     when,
	}
	return checkReconcileConditions(`CRD "bucket"`, rc, types.KELFunctionRegistry{Definitions: notes})
}

func TestReconcileConditions_CRAndArgsFromTheCR(t *testing.T) {
	require.NoError(t, reconcileWithConditions([]types.Condition{
		{Field: "spec.enabled", Equals: "true"},
		{Field: "args.quotaGB", GreaterThan: "0"},
	}))
}

func TestReconcileConditions_ExternalRejected(t *testing.T) {
	err := reconcileWithConditions([]types.Condition{{Field: "external.plan.status", Equals: "200"}})
	assert.ErrorContains(t, err, "reads .external")
	assert.ErrorContains(t, err, "preReconcile.reconcileGate")
}

func TestReconcileConditions_ExternalInAnOperandRejected(t *testing.T) {
	err := reconcileWithConditions([]types.Condition{{Field: "spec.tier", Equals: "{{ .external.plan.json.tier }}"}})
	assert.ErrorContains(t, err, "reads .external")
}

func TestReconcileConditions_ExternalThroughANoteRejected(t *testing.T) {
	err := reconcileWithConditions(
		[]types.Condition{{Field: "{{ planReady }}", Equals: "true"}},
		types.UserDefinedFunction{Name: "planReady", Expression: `{{ eq .external.plan.status "200" }}`},
	)
	assert.ErrorContains(t, err, "reads .external")
}

func TestReconcileConditions_ArgReadingExternalRejected(t *testing.T) {
	err := reconcileWithConditions([]types.Condition{{Field: "args.plan.tier", Equals: "gold"}})
	assert.ErrorContains(t, err, "reads args.plan")
}
