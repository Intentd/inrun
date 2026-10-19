package blueprint

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const profileModulePath = "../registry/module/testdata/with-profiles.yaml"

// expandBlueprintImports merges profiles from spec.imports into k.Profiles.
func TestExpandBlueprintImports_MergesProfiles(t *testing.T) {
	bp := &Blueprint{
		Spec: types.BlueprintSpec{
			Imports: []types.ModuleImport{
				{Module: profileModulePath},
			},
		},
		enabledCRDs: map[string]types.CRDEntry{},
	}

	require.NoError(t, bp.expandBlueprintImports())
	assert.Len(t, bp.Profiles.Resources, 1)
	assert.Equal(t, "org-standard", bp.Profiles.Resources[0].Name)
}

// Profiles from spec.crds[name].imports are ignored — only resources/admission merge there.
func TestExpandModuleImports_DoesNotMergeProfiles(t *testing.T) {
	bp := &Blueprint{
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				OperatorBox: &types.OperatorBoxConfig{
					Reconcile: &types.ReconcileConfig{
						Imports: []types.ModuleImport{
							{Module: profileModulePath},
						},
					},
				},
			},
		},
	}

	require.NoError(t, bp.expandModuleImports())
	assert.True(t, bp.Profiles.Empty(), "profiles must not be merged from CRD-level imports")

	// Resources from the module are still merged into the CRD's onReconcile.
	entry := bp.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}

// Both import levels used together: profiles from spec.imports, resources from CRD imports.
func TestExpandImports_BothLevels(t *testing.T) {
	bp := &Blueprint{
		Spec: types.BlueprintSpec{
			Imports: []types.ModuleImport{
				{Module: profileModulePath},
			},
		},
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				OperatorBox: &types.OperatorBoxConfig{
					Reconcile: &types.ReconcileConfig{
						Imports: []types.ModuleImport{
							{Module: profileModulePath},
						},
					},
				},
			},
		},
	}

	require.NoError(t, bp.expandBlueprintImports())
	require.NoError(t, bp.expandModuleImports())

	// Profiles come from spec.imports.
	assert.Len(t, bp.Profiles.Resources, 1)

	// Resources come from CRD-level import.
	entry := bp.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}
