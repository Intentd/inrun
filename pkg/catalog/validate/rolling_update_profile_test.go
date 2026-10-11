package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithDeployment(crdName string, deps ...types.DeploymentTemplateSource) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
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

func TestValidateRollingUpdateProfiles_NoCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_NoProfile(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{Name: "deploy"})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_ValidProfile_Safe(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		Name:          "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "safe"},
	})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_ValidProfile_Fast(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "fast"},
	})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_ValidProfile_BlueGreen(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "blue-green"},
	})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_UnknownProfile(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		Name:          "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "canary"},
	})
	err := e.validateRollingUpdateProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
	assert.Contains(t, err.Error(), "canary")
}

func TestValidateRollingUpdateProfiles_MixedWithMaxSurge(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		Name: "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{
			Profile:  "safe",
			MaxSurge: "1",
		},
	})
	err := e.validateRollingUpdateProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxSurge/maxUnavailable")
}

func TestValidateRollingUpdateProfiles_MixedWithMaxUnavailable(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		Name: "deploy",
		RollingUpdate: &types.RollingUpdateBehavior{
			Profile:        "fast",
			MaxUnavailable: "0",
		},
	})
	err := e.validateRollingUpdateProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxSurge/maxUnavailable")
}

func TestValidateRollingUpdateProfiles_TemplateExprSkipped(t *testing.T) {
	e := catalogWithDeployment("app", types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Profile: "{{ .Spec.RollingProfile }}"},
	})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}

func TestValidateRollingUpdateProfiles_StatefulSet(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						StatefulSets: []types.StatefulSetTemplateSource{
							{Name: "db", RollingUpdate: &types.RollingUpdateBehavior{Profile: "safe"}},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, e.validateRollingUpdateProfiles())
}
