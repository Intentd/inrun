package catalog

import (
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func objForTest() domain.Object {
	return domain.UnstructuredForTest()
}

func catalogWithPreReconcile(pr types.PreReconcileConfig) *Catalog {
	k := &Catalog{
		enabledCRDs: map[string]types.CRDEntry{
			gvkForTest(): {
				APITypes: types.APITypes{
					Kind:    "Application",
					Version: "v1",
					Group:   "test.inrun.catalog",
				},
				OperatorBox: &types.OperatorBoxConfig{
					PreReconcile: &pr,
				},
			},
		},
	}

	k.SetDefaults(config.NewDefaultConfig())
	k.SetGroupVersionKind()

	return k
}

func gvkForTest() string {
	return "test.inrun.catalog/v1, Kind=Application"
}

func TestIsEventAware_InitialState(t *testing.T) {
	k := catalogWithPreReconcile(types.PreReconcileConfig{
		ReconcileGate: &types.GateConditions{
			EventAware: true,
		},
	})

	obj := objForTest()
	gvk := gvkForTest()

	box := k.effectiveBox(obj, gvk)
	require.NotNil(t, box)
	require.NotNil(t, box.PreReconcile)
	require.NotNil(t, box.PreReconcile.ReconcileGate)

	t.Logf("has gate: %v", box.PreReconcile.HasReconcileGate())
	t.Logf("event aware: %v", box.PreReconcile.ReconcileGate.IsEventAware())

	assert.True(t, k.IsEventAware(obj, gvk))
}

func TestIsEventAware(t *testing.T) {
	tests := []struct {
		name     string
		catalog  *Catalog
		gvk      string
		expected bool
	}{
		{
			name: "event aware gate",
			catalog: catalogWithPreReconcile(types.PreReconcileConfig{
				ReconcileGate: &types.GateConditions{
					EventAware: true,
				},
			}),
			gvk:      gvkForTest(),
			expected: true,
		},
		{
			name: "event aware disabled",
			catalog: catalogWithPreReconcile(types.PreReconcileConfig{
				ReconcileGate: &types.GateConditions{
					EventAware: false,
				},
			}),
			gvk:      gvkForTest(),
			expected: false,
		},
		{
			name: "reconcile gate without event awareness",
			catalog: catalogWithPreReconcile(types.PreReconcileConfig{
				ReconcileGate: &types.GateConditions{
					When: []types.Condition{
						{
							Field:  "{{ .metadata.name }}",
							Equals: "app",
						},
					},
				},
			}),
			gvk:      gvkForTest(),
			expected: false,
		},
		{
			name:     "unknown gvk",
			catalog:  catalogWithPreReconcile(types.PreReconcileConfig{}),
			gvk:      "does-not-exist",
			expected: false,
		},
		{
			name:     "nil catalog",
			catalog:  nil,
			gvk:      gvkForTest(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool

			if tt.catalog == nil {
				got = (*Catalog)(nil).IsEventAware(objForTest(), tt.gvk)
			} else {
				got = tt.catalog.IsEventAware(objForTest(), tt.gvk)
			}

			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetPreReconcileSentinels_ReturnsDeclared(t *testing.T) {
	k := catalogWithPreReconcile(types.PreReconcileConfig{
		Sentinels: []string{
			"generationChanged",
			"labelsChanged",
		},
	})

	sentinels := k.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Equal(t, []string{"generationChanged", "labelsChanged"}, sentinels)
}

func TestGetPreReconcileSentinels_NoSentinels(t *testing.T) {
	k := catalogWithPreReconcile(types.PreReconcileConfig{})

	sentinels := k.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Empty(t, sentinels)
}

func TestGetPreReconcileSentinels_Unknown(t *testing.T) {
	k := catalogWithPreReconcile(types.PreReconcileConfig{
		Sentinels: []string{"generationChanged"},
	})

	sentinels := k.GetPreReconcileSentinels(objForTest(), "unknown")

	assert.Nil(t, sentinels)
}

func TestGetPreReconcileSentinels_NilCatalog(t *testing.T) {
	var k *Catalog

	sentinels := k.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Nil(t, sentinels)
}

func TestEffectiveBox_ResolvesTargetSpecificOperatorBox(t *testing.T) {
	targetEventAware := true
	crdEventAware := false

	k := &Catalog{
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				APITypes: types.APITypes{
					Kind:    "Application",
					Version: "v1",
					Group:   "test.inrun.catalog",
				},
				OperatorBox: &types.OperatorBoxConfig{
					PreReconcile: &types.PreReconcileConfig{
						ReconcileGate: &types.GateConditions{
							EventAware: crdEventAware,
						},
					},
				},
				Serve: &types.ServeConfig{
					Enabled: true,
					Target: types.ServeTargetValue{
						Entries: map[string]*types.ServeTargetConfig{
							"canary": {
								OperatorBox: &types.OperatorBoxConfig{
									PreReconcile: &types.PreReconcileConfig{
										ReconcileGate: &types.GateConditions{
											EventAware: targetEventAware,
										},
									}},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, k.SetGroupVersionKind())

	obj := objForTest()
	obj.SetAnnotations(map[string]string{"inrun.dev/serve-target": "canary"})

	box := k.effectiveBox(obj, gvkForTest())

	require.NotNil(t, box)
	require.NotNil(t, box.PreReconcile)
	require.NotNil(t, box.PreReconcile.ReconcileGate)

	assert.True(t, box.PreReconcile.ReconcileGate.IsEventAware())
}
