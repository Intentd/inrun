package blueprint_test

import (
	"reflect"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cfg = config.NewDefaultConfig()

func blueprintWithMetadata(m types.BlueprintMeta) *blueprint.Blueprint {
	return blueprint.NewBlueprintWithMetadataForTest(m, map[string]types.CRDEntry{
		"app": {IsStatusless: true},
	})
}

func blueprintWithAPITypes(a types.APITypes) *blueprint.Blueprint {
	return blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"app": {APITypes: a},
	})

}

func blueprintWithOperatorBox(box *types.OperatorBoxConfig) *blueprint.Blueprint {
	return blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"app": {OperatorBox: box},
	})

}

// setDefaults

func TestSetDefaults_MetadataNoData(t *testing.T) {
	e := blueprintWithMetadata(types.BlueprintMeta{})
	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.EnabledCRDs()["app"]
	assert.Equal(t, entry.BlueprintName, "inrun-runtime-app")
	assert.Equal(t, entry.BlueprintNamespace, "default")
}

func TestSetDefaults_MetadataInvalidName(t *testing.T) {
	e := blueprintWithMetadata(types.BlueprintMeta{
		Name:        "test:m,",
		Namespace:   "test-tea",
		ClusterName: "test-cluster",
	})
	err := e.SetDefaults(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid metadata.Name")
}

func TestSetDefaults_MetadataInvalidNameSpace(t *testing.T) {
	e := blueprintWithMetadata(types.BlueprintMeta{
		Name:        "test",
		Namespace:   "test-tea/m",
		ClusterName: "test-cluster",
	})
	err := e.SetDefaults(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid metadata.Namespace")
}

func TestSetDefaults_MetadataWithData(t *testing.T) {
	e := blueprintWithMetadata(types.BlueprintMeta{
		Namespace:   "test-team2",
		ClusterName: "test-cluster",
	})
	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.EnabledCRDs()["app"]
	assert.Equal(t, entry.BlueprintName, "test-cluster-app")
	assert.Equal(t, entry.BlueprintNamespace, "test-team2")
}

func TestSetDefaults_MetadataBlueprintNameDefaultToClusterName(t *testing.T) {
	e := blueprintWithMetadata(types.BlueprintMeta{
		Name:        "test",
		Namespace:   "test-team",
		ClusterName: "test-cluster",
	})
	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.EnabledCRDs()["app"]
	assert.Equal(t, entry.BlueprintName, "test")
	assert.Equal(t, entry.BlueprintNamespace, "test-team")
}

func TestSetDefaults_APITypes(t *testing.T) {
	e := blueprintWithAPITypes(types.APITypes{
		Kind:    "Website",
		Group:   "test.inrun.blueprint",
		Version: "v1",
	})

	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	// Assert APITypes default
	entry := e.EnabledCRDs()["app"]
	apiPath := entry.APITypes.APIPath
	plural := entry.APITypes.Plural

	assert.Contains(t, apiPath, "/apis")
	assert.Contains(t, plural, "websites")

}

func TestSetDefaults_OperatorBoxFinalizersDefault(t *testing.T) {
	e := blueprintWithOperatorBox(&types.OperatorBoxConfig{})
	e.Spec.Finalizers = []string{"spec-finalizer"}

	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	// The spec's finalizers, plus Inrun's cleanup finalizer on every CRD.
	entry := e.EnabledCRDs()["app"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	assert.ElementsMatch(t, []string{"spec-finalizer", labels.CleanupFinalizer}, boxFinalizers)
}

func TestSetDefaults_SpecFinalizerAddToOperatorBoxFinalizer(t *testing.T) {
	e := blueprintWithOperatorBox(&types.OperatorBoxConfig{})
	e.Spec.Finalizers = []string{"spec-finalizer"}

	err := e.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.EnabledCRDs()["app"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	boxFinalizers = append(boxFinalizers, "box-finalizer")

	if reflect.DeepEqual(boxFinalizers, e.Spec.Finalizers) {
		t.Fatalf("unexpected error. want false, got true")
	}

	if len(boxFinalizers) != 3 { // spec, cleanup, box
		t.Fatalf("unexpected error. want true, got false: %d", len(boxFinalizers))
	}
}

func TestSetDefaults_TargetOperatorBoxFinalizersDefault(t *testing.T) {
	serve := &types.ServeConfig{
		Enabled: true,
		Target: types.ServeTargetValue{
			Entries: map[string]*types.ServeTargetConfig{
				"testfixture": {
					OperatorBox: &types.OperatorBoxConfig{
						Runtime: &types.RuntimeConfig{Finalizers: &types.FinalizerConfig{
							Names: []string{"target-finalizer"},
						}},
					},
				},
			},
		},
	}

	e := blueprint.NewBlueprintForTestWithSpec(
		map[string]types.CRDEntry{
			"myresource": {
				APITypes: types.APITypes{
					Group:   "demo.inrun.dev",
					Version: "v1",
					Kind:    "Myresource",
					Plural:  "myresources",
				},
				Serve: serve,
			},
		},
		types.BlueprintSpec{Finalizers: []string{"spec-finalizer"}},
	)

	entry := e.EnabledCRDs()["myresource"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	if reflect.DeepEqual(boxFinalizers, e.Spec.Finalizers) {
		t.Fatalf("unexpected error. want true, got false")
	}

	if len(boxFinalizers) != 3 { // spec, target, cleanup
		t.Fatalf("unexpected boxFinalizers length: want 3, got %d", len(boxFinalizers))
	}

	targetFinalizers := entry.Serve.Target.Entries["testfixture"].OperatorBox.EffectiveFinalizers()
	if len(targetFinalizers) != 1 {
		t.Fatalf("unexpected error. want true, got false")
	}
}
