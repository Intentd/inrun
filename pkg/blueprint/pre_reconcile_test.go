package blueprint

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

func blueprintWithPreReconcile(pr types.PreReconcileConfig) *Blueprint {
	bp := &Blueprint{
		enabledCRDs: map[string]types.CRDEntry{
			gvkForTest(): {
				APITypes: types.APITypes{
					Kind:    "Application",
					Version: "v1",
					Group:   "test.inrun.blueprint",
				},
				OperatorBox: &types.OperatorBoxConfig{
					PreReconcile: &pr,
				},
			},
		},
	}

	bp.SetDefaults(config.NewDefaultConfig())
	bp.SetGroupVersionKind()

	return bp
}

func gvkForTest() string {
	return "test.inrun.blueprint/v1, Kind=Application"
}

func TestIsEventAware_InitialState(t *testing.T) {
	bp := blueprintWithPreReconcile(types.PreReconcileConfig{
		ReconcileGate: &types.GateConditions{
			EventAware: true,
		},
	})

	obj := objForTest()
	gvk := gvkForTest()

	box := bp.effectiveBox(obj, gvk)
	require.NotNil(t, box)
	require.NotNil(t, box.PreReconcile)
	require.NotNil(t, box.PreReconcile.ReconcileGate)

	t.Logf("has gate: %v", box.PreReconcile.HasReconcileGate())
	t.Logf("event aware: %v", box.PreReconcile.ReconcileGate.IsEventAware())

	assert.True(t, bp.IsEventAware(obj, gvk))
}

func TestIsEventAware(t *testing.T) {
	tests := []struct {
		name      string
		blueprint *Blueprint
		gvk       string
		expected  bool
	}{
		{
			name: "event aware gate",
			blueprint: blueprintWithPreReconcile(types.PreReconcileConfig{
				ReconcileGate: &types.GateConditions{
					EventAware: true,
				},
			}),
			gvk:      gvkForTest(),
			expected: true,
		},
		{
			name: "event aware disabled",
			blueprint: blueprintWithPreReconcile(types.PreReconcileConfig{
				ReconcileGate: &types.GateConditions{
					EventAware: false,
				},
			}),
			gvk:      gvkForTest(),
			expected: false,
		},
		{
			name: "reconcile gate without event awareness",
			blueprint: blueprintWithPreReconcile(types.PreReconcileConfig{
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
			name:      "unknown gvk",
			blueprint: blueprintWithPreReconcile(types.PreReconcileConfig{}),
			gvk:       "does-not-exist",
			expected:  false,
		},
		{
			name:      "nil blueprint",
			blueprint: nil,
			gvk:       gvkForTest(),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool

			if tt.blueprint == nil {
				got = (*Blueprint)(nil).IsEventAware(objForTest(), tt.gvk)
			} else {
				got = tt.blueprint.IsEventAware(objForTest(), tt.gvk)
			}

			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetPreReconcileSentinels_ReturnsDeclared(t *testing.T) {
	bp := blueprintWithPreReconcile(types.PreReconcileConfig{
		Sentinels: []string{
			"generationChanged",
			"labelsChanged",
		},
	})

	sentinels := bp.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Equal(t, []string{"generationChanged", "labelsChanged"}, sentinels)
}

func TestGetPreReconcileSentinels_NoSentinels(t *testing.T) {
	bp := blueprintWithPreReconcile(types.PreReconcileConfig{})

	sentinels := bp.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Empty(t, sentinels)
}

func TestGetPreReconcileSentinels_Unknown(t *testing.T) {
	bp := blueprintWithPreReconcile(types.PreReconcileConfig{
		Sentinels: []string{"generationChanged"},
	})

	sentinels := bp.GetPreReconcileSentinels(objForTest(), "unknown")

	assert.Nil(t, sentinels)
}

func TestGetPreReconcileSentinels_NilBlueprint(t *testing.T) {
	var bp *Blueprint

	sentinels := bp.GetPreReconcileSentinels(objForTest(), gvkForTest())

	assert.Nil(t, sentinels)
}

func TestEffectiveBox_ResolvesTargetSpecificOperatorBox(t *testing.T) {
	targetEventAware := true
	crdEventAware := false

	bp := &Blueprint{
		enabledCRDs: map[string]types.CRDEntry{
			"app": {
				APITypes: types.APITypes{
					Kind:    "Application",
					Version: "v1",
					Group:   "test.inrun.blueprint",
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

	require.NoError(t, bp.SetGroupVersionKind())

	obj := objForTest()
	obj.SetAnnotations(map[string]string{"inrun.dev/serve-target": "canary"})

	box := bp.effectiveBox(obj, gvkForTest())

	require.NotNil(t, box)
	require.NotNil(t, box.PreReconcile)
	require.NotNil(t, box.PreReconcile.ReconcileGate)

	assert.True(t, box.PreReconcile.ReconcileGate.IsEventAware())
}
