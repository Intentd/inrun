package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithResourceQuotaOnCreate(rqs ...types.ResourceQuotaTemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					ResourceQuotas: rqs,
				},
			},
		},
	}
}

func TestCollectResourceQuotaPresetEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectResourceQuotaPresetEntries())
}

func TestCollectResourceQuotaPresetEntries_NoPreset(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{Name: "quota"})
	assert.Empty(t, c.CollectResourceQuotaPresetEntries())
}

func TestCollectResourceQuotaPresetEntries_PresetReturned(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Name:   "default-quota",
		Preset: "small",
	})
	entries := c.CollectResourceQuotaPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "default-quota", entries[0].ResourceName)
	assert.Equal(t, "small", entries[0].Preset)
	assert.False(t, entries[0].Mixed)
}

func TestCollectResourceQuotaPresetEntries_Mixed(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Name:   "quota",
		Preset: "medium",
		Hard:   map[string]string{"pods": "5"},
	})
	entries := c.CollectResourceQuotaPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectResourceQuotaPresetEntries_TemplateExpr(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Preset: "{{ .Spec.QuotaPreset }}",
	})
	entries := c.CollectResourceQuotaPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.QuotaPreset }}", entries[0].Preset)
}

func TestCollectResourceQuotaPresetEntries_MultiplePresets(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					ResourceQuotas: []types.ResourceQuotaTemplateSource{
						{Name: "rq-small", Preset: "small"},
						{Name: "rq-no-preset"},
						{Name: "rq-large", Preset: "large"},
					},
				},
			},
		},
	}
	entries := c.CollectResourceQuotaPresetEntries()
	require.Len(t, entries, 2)
	assert.Equal(t, "small", entries[0].Preset)
	assert.Equal(t, "large", entries[1].Preset)
}

func TestCollectResourceQuotaPresetEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					ResourceQuotas: []types.ResourceQuotaTemplateSource{
						{Name: "quota", Preset: "xlarge"},
					},
				},
			},
		},
	}
	entries := c.CollectResourceQuotaPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "xlarge", entries[0].Preset)
}
