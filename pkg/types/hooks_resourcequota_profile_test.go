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

func TestCollectResourceQuotaProfileEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectResourceQuotaProfileEntries())
}

func TestCollectResourceQuotaProfileEntries_NoProfile(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{Name: "quota"})
	assert.Empty(t, c.CollectResourceQuotaProfileEntries())
}

func TestCollectResourceQuotaProfileEntries_ProfileReturned(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Name:    "default-quota",
		Profile: "small",
	})
	entries := c.CollectResourceQuotaProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "default-quota", entries[0].ResourceName)
	assert.Equal(t, "small", entries[0].Profile)
	assert.False(t, entries[0].Mixed)
}

func TestCollectResourceQuotaProfileEntries_Mixed(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Name:    "quota",
		Profile: "medium",
		Hard:    map[string]string{"pods": "5"},
	})
	entries := c.CollectResourceQuotaProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectResourceQuotaProfileEntries_TemplateExpr(t *testing.T) {
	c := crdWithResourceQuotaOnCreate(types.ResourceQuotaTemplateSource{
		Profile: "{{ .Spec.QuotaProfile }}",
	})
	entries := c.CollectResourceQuotaProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.QuotaProfile }}", entries[0].Profile)
}

func TestCollectResourceQuotaProfileEntries_MultipleProfiles(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					ResourceQuotas: []types.ResourceQuotaTemplateSource{
						{Name: "rq-small", Profile: "small"},
						{Name: "rq-no-profile"},
						{Name: "rq-large", Profile: "large"},
					},
				},
			},
		},
	}
	entries := c.CollectResourceQuotaProfileEntries()
	require.Len(t, entries, 2)
	assert.Equal(t, "small", entries[0].Profile)
	assert.Equal(t, "large", entries[1].Profile)
}

func TestCollectResourceQuotaProfileEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					ResourceQuotas: []types.ResourceQuotaTemplateSource{
						{Name: "quota", Profile: "xlarge"},
					},
				},
			},
		},
	}
	entries := c.CollectResourceQuotaProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "xlarge", entries[0].Profile)
}
