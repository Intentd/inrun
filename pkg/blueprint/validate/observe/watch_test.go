package observe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithWatch(crdName string, box types.OperatorBoxConfig) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {OperatorBox: &box},
	})
}

func TestValidateWatchEntries_Empty(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateWatchEntries_Valid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "apps/v1", Kind: "Deployment"},
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "shared-config"},
				{APIVersion: "v1", Kind: "Node", On: []string{"update"}},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateWatchEntries_MissingAPIVersion(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{Kind: "Deployment"},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiVersion must not be empty")
}

func TestValidateWatchEntries_MissingKind(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "apps/v1"},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind must not be empty")
}

func TestValidateWatchEntries_InvalidOnValue(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "apps/v1", Kind: "Deployment", On: []string{"modified"}},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modified")
	assert.Contains(t, err.Error(), "create, update, delete")
}

func TestValidateWatchEntries_DuplicateEntry(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "apps/v1", Kind: "Deployment"},
				{APIVersion: "apps/v1", Kind: "Deployment"},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate watch entry")
}

func TestValidateWatchEntries_DuplicateWithDifferentNamespace(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "ns-a"},
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "ns-b"},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateWatchEntries_ValidAllOnValues(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "Node", On: []string{"create", "update", "delete"}},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateSentinels_Valid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged"},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateSentinels_Unknown(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "specChanged"},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specChanged")
	assert.Contains(t, err.Error(), "generationChanged, labelsChanged, annotationsChanged")
}

func TestValidateSentinels_AllValid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged", "annotationsChanged"},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateSentinels_EnqueueGateValid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged", "annotationsChanged"},
			EnqueueGate: &types.GateConditions{
				Sentinels: []string{"generationChanged", "labelsChanged"},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateSentinels_EnqueueGateInValid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged", "annotationsChanged"},
			EnqueueGate: &types.GateConditions{
				Sentinels: []string{"generationChanging", "labelsChanged"},
			},
		},
	})
	assert.Error(t, e.validateWatchEntries())
}

func TestValidateSentinels_ReconcileGateValid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged", "annotationsChanged"},
			ReconcileGate: &types.GateConditions{
				Sentinels: []string{"generationChanged", "labelsChanged"},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateSentinels_ReconcileGateInValid(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged", "labelsChanged", "annotationsChanged"},
			ReconcileGate: &types.GateConditions{
				Sentinels: []string{"generationChanging", "labelsChanged"},
			},
		},
	})
	assert.Error(t, e.validateWatchEntries())
}

func TestValidateGateTemplate_DeclaredSentinelParsesOK(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged"},
			EnqueueGate: &types.GateConditions{
				When: []types.Condition{
					{Field: `{{ generationChanged }}`, Equals: "true"},
				},
			},
		},
	})
	assert.NoError(t, e.validateWatchEntries())
}

func TestValidateGateTemplate_UndeclaredSentinelFails(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		PreReconcile: &types.PreReconcileConfig{
			Sentinels: []string{"generationChanged"},
			EnqueueGate: &types.GateConditions{
				When: []types.Condition{
					{Field: `{{ labelsChanged }}`, Equals: "true"},
				},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateKeyFrom_ValidLabel(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "ConfigMap", KeyFrom: &types.WatchKeyFrom{Label: "app.kubernetes.io/cr-owner"}},
			},
		},
	})
	require.NoError(t, e.validateWatchEntries())
}

func TestValidateKeyFrom_ValidName(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "Node", KeyFrom: &types.WatchKeyFrom{Name: "my-singleton"}},
			},
		},
	})
	require.NoError(t, e.validateWatchEntries())
}

func TestValidateKeyFrom_BothLabelAndName(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "ConfigMap", KeyFrom: &types.WatchKeyFrom{Label: "some-label", Name: "some-name"}},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one of label or name")
}

func TestValidateKeyFrom_NeitherLabelNorName(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "ConfigMap", KeyFrom: &types.WatchKeyFrom{}},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither label nor name")
}

func TestValidateKeyFrom_NamespaceWithLabelRejected(t *testing.T) {
	e := blueprintWithWatch("myapp", types.OperatorBoxConfig{
		Observe: &types.Observe{
			Watch: []types.WatchEntry{
				{APIVersion: "v1", Kind: "ConfigMap", KeyFrom: &types.WatchKeyFrom{Label: "some-label", Namespace: "default"}},
			},
		},
	})
	err := e.validateWatchEntries()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace has no effect")
}
