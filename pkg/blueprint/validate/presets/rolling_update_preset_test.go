package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithDeployment(crdName string, deps ...types.DeploymentTemplateSource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						Deployments: deps,
					},
				},
			},
		},
	})
}

func TestValidateRollingUpdatePresets_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_NoPreset(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{Name: "deploy"})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_ValidPreset_Safe(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		Name:          "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "safe"},
	})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_ValidPreset_Fast(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "fast"},
	})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_ValidPreset_BlueGreen(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "blue-green"},
	})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_UnknownPreset(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		Name:          "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "canary"},
	})
	err := e.validateRollingUpdatePresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
	assert.Contains(t, err.Error(), "canary")
}

func TestValidateRollingUpdatePresets_MixedWithMaxSurge(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		Name: "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{
			Preset:   "safe",
			MaxSurge: "1",
		},
	})
	err := e.validateRollingUpdatePresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxSurge/maxUnavailable")
}

func TestValidateRollingUpdatePresets_MixedWithMaxUnavailable(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		Name: "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{
			Preset:         "fast",
			MaxUnavailable: "0",
		},
	})
	err := e.validateRollingUpdatePresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxSurge/maxUnavailable")
}

func TestValidateRollingUpdatePresets_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "{{ .Spec.RollingPreset }}"},
	})
	assert.NoError(t, e.validateRollingUpdatePresets())
}

func TestValidateRollingUpdatePresets_StatefulSet(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						StatefulSets: []types.StatefulSetTemplateSource{
							{Name: "db", RollingUpdate: &types.RollingUpdateBehavior{Preset: "safe"}},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, e.validateRollingUpdatePresets())
}
