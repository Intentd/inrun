package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithDeploymentOnCreate(deps ...types.DeploymentTemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					Deployments: deps,
				},
			},
		},
	}
}

func TestCollectRollingUpdateProfileEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectRollingUpdateProfileEntries())
}

func TestCollectRollingUpdateProfileEntries_NoRollingUpdate(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{Name: "app"})
	assert.Empty(t, c.CollectRollingUpdateProfileEntries())
}

func TestCollectRollingUpdateProfileEntries_RollingUpdateNoProfile(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name:          "app",
		RollingUpdate: &types.RollingUpdateBehavior{},
	})
	assert.Empty(t, c.CollectRollingUpdateProfileEntries())
}

func TestCollectRollingUpdateProfileEntries_ProfileReturned(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name:          "app",
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "safe"},
	})
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "app", entries[0].ResourceName)
	assert.Equal(t, "safe", entries[0].Profile)
	assert.False(t, entries[0].Mixed)
}

func TestCollectRollingUpdateProfileEntries_Mixed_MaxSurge(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name: "app",
		RollingUpdate: &types.RollingUpdateBehavior{
			Profile:  "fast",
			MaxSurge: "2",
		},
	})
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectRollingUpdateProfileEntries_Mixed_MaxUnavailable(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name: "app",
		RollingUpdate: &types.RollingUpdateBehavior{
			Profile:        "blue-green",
			MaxUnavailable: "0",
		},
	})
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectRollingUpdateProfileEntries_TemplateExpr(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "{{ .Spec.DeployProfile }}"},
	})
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.DeployProfile }}", entries[0].Profile)
}

func TestCollectRollingUpdateProfileEntries_StatefulSet(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					StatefulSets: []types.StatefulSetTemplateSource{
						{Name: "db", RollingUpdate: &types.RollingUpdateBehavior{Profile: "safe"}},
					},
				},
			},
		},
	}
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "safe", entries[0].Profile)
	assert.Equal(t, "db", entries[0].ResourceName)
}

func TestCollectRollingUpdateProfileEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					Deployments: []types.DeploymentTemplateSource{
						{Name: "app", RollingUpdate: &types.RollingUpdateBehavior{Profile: "fast"}},
					},
				},
			},
		},
	}
	entries := c.CollectRollingUpdateProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "fast", entries[0].Profile)
}
