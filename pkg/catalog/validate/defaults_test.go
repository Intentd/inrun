package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults_ModeFromLocation(t *testing.T) {
	cat := catalog.NewCatalogForTest(map[string]types.CRDEntry{
		"typed":   {APITypes: types.APITypes{Kind: "Typed", Location: "github.com/test/apis/v1"}},
		"dynamic": {APITypes: types.APITypes{Kind: "Dynamic"}},
	})
	require.NoError(t, applyDefaults(cat))
	assert.Equal(t, types.CRDModeTyped, cat.EnabledCRDs()["typed"].Mode)
	assert.Equal(t, types.CRDModeDynamic, cat.EnabledCRDs()["dynamic"].Mode)
}

func TestApplyDefaults_DropAlwaysOnAtThreshold(t *testing.T) {
	no := false
	cat := catalog.NewCatalogForTest(map[string]types.CRDEntry{
		"app": {OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Queue: types.Queue{
			MaxDepth: 100,
			Cfg: &types.QueueBehaviour{
				OnThreshold: &types.QueueBehaviourSetting{Drop: &no, Value: 5},
			},
		}}}},
	})
	require.NoError(t, applyDefaults(cat))
	entry := cat.EnabledCRDs()["app"]
	assert.True(t, *entry.QueueConfig().Behaviour().OnThreshold.Drop)
}
