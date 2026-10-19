package observe

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type crdInfo struct {
	name    string
	group   string
	kind    string
	version string
	label   map[string]string
	box     *types.OperatorBoxConfig
}

func crdForTest(info *crdInfo) types.CRDEntry {
	if info == nil {
		info = &crdInfo{
			name:    "web",
			kind:    "Web",
			group:   "web.io",
			version: "v1",
		}
	}

	return types.CRDEntry{
		Name: info.name,
		APITypes: types.APITypes{
			Kind:    info.kind,
			Group:   info.group,
			Version: info.version,
		}, Labels: info.label, OperatorBox: info.box,
	}
}

func crossBox(decls ...types.CrossCRDDeclaration) *types.OperatorBoxConfig {
	return &types.OperatorBoxConfig{
		Observe: &types.Observe{
			Cross: decls,
		},
	}
}

func TestValidateCrossDecl_Nil(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(),
		},
	})
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_ValidWithCRDOnly(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1"}),
		},
		"app1": {},
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_ValidMultipleWithCRDOnly(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": crdForTest(&crdInfo{name: "app", box: crossBox(
			types.CrossCRDDeclaration{CRD: "app1"},
			types.CrossCRDDeclaration{CRD: "app2"},
		)}),
		"app1": crdForTest(&crdInfo{name: "app1", box: crossBox(
			types.CrossCRDDeclaration{CRD: "app"},
			types.CrossCRDDeclaration{CRD: "app2"},
		)}),
		"app2": crdForTest(&crdInfo{name: "app2"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_ValidLabelsOnly(t *testing.T) {
	sel := map[string]string{"test": "hello"}
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{
				LabelSelector: sel,
				As:            "ap",
			}),
		},
		"app1": crdForTest(&crdInfo{label: sel}),
	})

	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_InvalidLabelsAndCRD(t *testing.T) {
	sel := map[string]string{"test": "hello"}
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{
				LabelSelector: sel,
				As:            "ap",
				CRD:           "app1",
			}),
		},
		"app1": crdForTest(&crdInfo{label: sel}),
	})

	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "only one of 'crd:' or 'labels:' is required.")
}

func TestValidateCrossDecl_InvalidLabelsOnly(t *testing.T) {
	sel := map[string]string{"test": "hello"}
	sel2 := map[string]string{"test": "hello2"}
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": crdForTest(&crdInfo{name: "app", box: crossBox(types.CrossCRDDeclaration{
			LabelSelector: sel,
			As:            "ap",
		})}),
		"app2": crdForTest(&crdInfo{label: sel2}),
	})

	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found in any CRD in the blueprint")
}

func TestValidateCrossDecl_InvalidNoCRDAndLabels(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{As: "no-crd-or-label"}),
		},
	})
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "crd or labels required")
}

func TestValidateCrossDecl_InvalidLabelsNoAs(t *testing.T) {
	sel := map[string]string{"test": "hello"}
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{LabelSelector: sel}),
		},
		"app1": crdForTest(&crdInfo{label: sel}),
	})

	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "'as' is required for cross referencing")
}

func TestValidateCrossDecl_InvalidSelfReferencing(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app"}),
		},
	})
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "cross: a CRD cannot reference itself")
}

func TestValidateCrossDecl_WithInvalidCRDNoSource(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app-unknown"}),
		},
	})
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "\"app-unknown\" not found")
}

func TestValidateCrossDecl_WithValidSource(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{
				CRD: "app-unknown",
				Source: &types.CrossSource{
					Host:     "http://localhost:8080",
					Protocol: "cr",
				},
			}),
		},
	})
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_MultipleWithInvalidCRD(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(
				types.CrossCRDDeclaration{CRD: "app-unknown"},
				types.CrossCRDDeclaration{CRD: "app2"},
				types.CrossCRDDeclaration{CRD: "app-unknown3"},
			),
		},
		"app2": crdForTest(&crdInfo{name: "app2"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "\"app-unknown\" not found")
}

func TestValidateCrossDecl_ValidWithAs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", As: "ap"}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_InvalidAs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(
				types.CrossCRDDeclaration{CRD: "app1", As: "ap/"},
				types.CrossCRDDeclaration{CRD: "app2", As: "ap"},
			),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
		"app2": crdForTest(&crdInfo{name: "app2"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid 'as' name")
}

func TestValidateCrossDecl_InvalidDuplicateAs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(
				types.CrossCRDDeclaration{CRD: "app1", As: "ap"},
				types.CrossCRDDeclaration{CRD: "app2", As: "ap"},
			),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
		"app2": crdForTest(&crdInfo{name: "app2"}),
		"web": {
			OperatorBox: crossBox(
				types.CrossCRDDeclaration{CRD: "app1", As: "ap"},
				types.CrossCRDDeclaration{CRD: "app2", As: "ap"},
			),
		},
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
}

func TestValidateCrossDecl_ValidProtocols(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app4", Source: &types.CrossSource{
				Protocol: "info",
			}}),
		},
		"app1": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app", Source: &types.CrossSource{Protocol: "metrics"}}),
		},
		"app2": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app", Source: &types.CrossSource{Protocol: "health"}}),
		},
		"app3": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app", Source: &types.CrossSource{Protocol: "health"}}),
		},
		"app4": {},
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_InvalidProtocols(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app4", Source: &types.CrossSource{
				Protocol: "info",
			}}),
		},
		"app1": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app", Source: &types.CrossSource{Protocol: "health-style"}}),
		},
		"app2": {},
		"app4": {},
	})

	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid ONCOP protocol")
}

func TestValidateCrossDecl_ValidTokenAuth(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					Token: "test-token",
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_ValidSecretRefAuth(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					SecretRef: &types.APISecretRef{
						Name:      "test-token",
						Namespace: "test-ns",
						Key:       "new-token",
					},
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)
}

func TestValidateCrossDecl_InvalidAuthWithTokenAndSecretRef(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					Token: "test-token",
					SecretRef: &types.APISecretRef{
						Name:      "test-token",
						Namespace: "test-ns",
						Key:       "new-token",
					},
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "declared both token and secretRef. Only one is allowed")
}

func TestValidateCrossDecl_InvalidSecretRefAuthNoName(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					SecretRef: &types.APISecretRef{
						Namespace: "test-ns",
						Key:       "new-token",
					},
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "name is required for secretRef")
}

func TestValidateCrossDecl_InvalidSecretRefAuthNoKey(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					SecretRef: &types.APISecretRef{
						Name:      "test-token",
						Namespace: "test-ns",
					},
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	require.Error(t, err)
	assert.ErrorContains(t, err, "key is required for secretRef")
}

func TestValidateCrossDecl_WarnAuthWithSecretRefNoNamespace(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{
		"app": {
			OperatorBox: crossBox(types.CrossCRDDeclaration{CRD: "app1", Source: &types.CrossSource{
				Auth: &types.Auth{
					SecretRef: &types.APISecretRef{
						Name: "test-token",
						Key:  "new-token",
					},
				},
			}}),
		},
		"app1": crdForTest(&crdInfo{name: "app1"}),
	})
	e.Blueprint.BuildLookupIndexes()
	err := e.validateCrossDecl()
	assert.NoError(t, err)

	entry := e.Blueprint.EnabledCRDs()["app"]
	if !entry.Warnings.HasWarnings() {
		t.Fatalf("expected crd to have warning")
	}
	assert.True(t, entry.Warnings.Contains("Defaults to inrun namespace"))
}
