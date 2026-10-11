package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithEmit(crdName string, emit *types.EmitConfig) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &types.OperatorBoxConfig{
			Emit: emit,
		}},
	})
}

func TestValidateEmit_NilEmit(t *testing.T) {
	k := catalogWithEmit("myapp", nil)
	assert.NoError(t, k.validateEmit())
}

func TestValidateEmit_NoEvents(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{})
	assert.NoError(t, k.validateEmit())
}

func TestValidateEmit_ValidNormal(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"DatabaseReady": {
				Type:    types.EmitEventTypeNormal,
				Reason:  "Ready",
				Message: "{{ .spec.name }} is ready",
			},
		},
	})
	assert.NoError(t, k.validateEmit())
}

func TestValidateEmit_ValidWarning(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"DatabaseFailed": {
				Type:    types.EmitEventTypeWarning,
				Reason:  "SyncFailed",
				Message: "{{ .spec.name }} failed to sync",
			},
		},
	})
	assert.NoError(t, k.validateEmit())
}

func TestValidateEmit_InvalidType(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"BadEvent": {
				Type:    "Critical",
				Reason:  "Oops",
				Message: "something went wrong",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
	assert.Contains(t, err.Error(), "Critical")
	assert.Contains(t, err.Error(), types.EmitEventTypesJoined())
}

func TestValidateEmit_EmptyType(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"NoType": {
				Reason:  "SomeReason",
				Message: "some message",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type")
}

func TestValidateEmit_EmptyReason(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"NoReason": {
				Type:    types.EmitEventTypeNormal,
				Message: "some message",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason must not be empty")
}

func TestValidateEmit_EmptyMessage(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"NoMessage": {
				Type:   types.EmitEventTypeWarning,
				Reason: "SyncFailed",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message must not be empty")
}

func TestValidateEmitEventEntry_DirectCall(t *testing.T) {
	err := validateEmitEventEntry("myapp", "TestEvent", &types.EmitEventEntry{
		Type:    types.EmitEventTypeNormal,
		Reason:  "Ready",
		Message: "ready",
	}, nil)
	assert.NoError(t, err)
}

func TestValidateEmit_InvalidMessageTemplate(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"BadTemplate": {
				Type:    types.EmitEventTypeNormal,
				Reason:  "Ready",
				Message: "{{ .spec.name",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateEmit_InvalidWhenFieldTemplate(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"BadWhen": {
				Type:    types.EmitEventTypeWarning,
				Reason:  "Failed",
				Message: "something failed",
				When: []types.Condition{
					{Field: "{{ .status.phase"},
				},
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateEmit_MultipleEntries_OneInvalid(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"GoodEvent": {
				Type:    types.EmitEventTypeNormal,
				Reason:  "Ready",
				Message: "{{ .spec.name }} is ready",
			},
			"BadEvent": {
				Type:    types.EmitEventTypeWarning,
				Reason:  "Failed",
				Message: "",
			},
		},
	})
	err := k.validateEmit()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message must not be empty")
}

func TestValidateEmit_WithWhenCondition(t *testing.T) {
	k := catalogWithEmit("myapp", &types.EmitConfig{
		Events: map[string]*types.EmitEventEntry{
			"PhaseReady": {
				Type:    types.EmitEventTypeNormal,
				Reason:  "Ready",
				Message: "{{ .spec.name }} is ready",
				When: []types.Condition{
					{Field: ".status.phase", Equals: "Ready"},
				},
			},
		},
	})
	assert.NoError(t, k.validateEmit())
}
