package catalog

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const profileModulePath = "../registry/module/testdata/with-profiles.yaml"

// expandCatalogImports merges profiles from spec.imports into k.Profiles.
func TestExpandCatalogImports_MergesProfiles(t *testing.T) {
	k := &Catalog{
		Spec: types.CatalogSpec{
			Imports: []types.ModuleImport{
				{Module: profileModulePath},
			},
		},
		enabledCRDs: map[string]types.CRDEntry{},
	}

	require.NoError(t, k.expandCatalogImports())
	assert.Len(t, k.Profiles.Resources, 1)
	assert.Equal(t, "org-standard", k.Profiles.Resources[0].Name)
}

// Profiles from spec.crds[name].imports are ignored — only resources/admission merge there.
func TestExpandModuleImports_DoesNotMergeProfiles(t *testing.T) {
	k := &Catalog{
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

	require.NoError(t, k.expandModuleImports())
	assert.True(t, k.Profiles.Empty(), "profiles must not be merged from CRD-level imports")

	// Resources from the module are still merged into the CRD's onReconcile.
	entry := k.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}

// Both import levels used together: profiles from spec.imports, resources from CRD imports.
func TestExpandImports_BothLevels(t *testing.T) {
	k := &Catalog{
		Spec: types.CatalogSpec{
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

	require.NoError(t, k.expandCatalogImports())
	require.NoError(t, k.expandModuleImports())

	// Profiles come from spec.imports.
	assert.Len(t, k.Profiles.Resources, 1)

	// Resources come from CRD-level import.
	entry := k.enabledCRDs["app"]
	require.NotNil(t, entry.OperatorBox.Reconcile)
	require.NotNil(t, entry.OperatorBox.Reconcile.OnReconcile)
	assert.NotEmpty(t, entry.OperatorBox.Reconcile.OnReconcile.Deployments)
}
