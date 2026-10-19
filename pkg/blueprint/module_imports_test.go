package blueprint

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const presetModulePath = "../registry/module/testdata/with-presets.yaml"

// expandBlueprintImports merges presets from spec.imports into k.Presets.
func TestExpandBlueprintImports_MergesPresets(t *testing.T) {
	bp := &Blueprint{
		Spec: types.BlueprintSpec{
			Imports: []types.ModuleImport{
				{Module: presetModulePath},
			},
		},
		enabledCRDs: map[string]types.CRDEntry{},
	}

	require.NoError(t, bp.expandBlueprintImports())
	assert.Len(t, bp.Presets.Resources, 1)
	assert.Equal(t, "org-standard", bp.Presets.Resources[0].Name)
}

// Presets from spec.crds[name].imports are ignored — only resources/admission merge there.
func TestExpandModuleImports_DoesNotMergePresets(t *testing.T) {
	bp := &Blueprint{
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				OperatorBox: &types.OperatorBoxConfig{
					Reconcile: &types.ReconcileConfig{
						Imports: []types.ModuleImport{
							{Module: presetModulePath},
						},
					},
				},
			},
		},
	}

	require.NoError(t, bp.expandModuleImports())
	assert.True(t, bp.Presets.Empty(), "presets must not be merged from CRD-level imports")

	// Resources from the module are still merged into the CRD's onReconcile.
	entry := bp.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}

// Both import levels used together: presets from spec.imports, resources from CRD imports.
func TestExpandImports_BothLevels(t *testing.T) {
	bp := &Blueprint{
		Spec: types.BlueprintSpec{
			Imports: []types.ModuleImport{
				{Module: presetModulePath},
			},
		},
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				OperatorBox: &types.OperatorBoxConfig{
					Reconcile: &types.ReconcileConfig{
						Imports: []types.ModuleImport{
							{Module: presetModulePath},
						},
					},
				},
			},
		},
	}

	require.NoError(t, bp.expandBlueprintImports())
	require.NoError(t, bp.expandModuleImports())

	// Presets come from spec.imports.
	assert.Len(t, bp.Presets.Resources, 1)

	// Resources come from CRD-level import.
	entry := bp.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}
