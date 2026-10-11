package validate

import (
	"reflect"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cfg = config.NewDefaultConfig()

func catalogWithMetadata(m types.CatalogMeta) *executor {
	return newExec(catalog.NewCatalogWithMetadataForTest(m, map[string]types.CRDEntry{
		"app": {IsStatusless: true},
	}))
}

func catalogWithAPITypes(a types.APITypes) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {APITypes: a},
	})

}

func catalogWithOperatorBox(box *types.OperatorBoxConfig) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"app": {OperatorBox: box},
	})

}

// setDefaults

func TestSetDefaults_MetadataNoData(t *testing.T) {
	e := catalogWithMetadata(types.CatalogMeta{})
	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.cat.EnabledCRDs()["app"]
	assert.Equal(t, entry.CatalogName, "inrun-runtime-app")
	assert.Equal(t, entry.CatalogNamespace, "default")
}

func TestSetDefaults_MetadataInvalidName(t *testing.T) {
	e := catalogWithMetadata(types.CatalogMeta{
		Name:        "test:m,",
		Namespace:   "test-tea",
		ClusterName: "test-cluster",
	})
	err := e.cat.SetDefaults(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid metadata.Name")
}

func TestSetDefaults_MetadataInvalidNameSpace(t *testing.T) {
	e := catalogWithMetadata(types.CatalogMeta{
		Name:        "test",
		Namespace:   "test-tea/m",
		ClusterName: "test-cluster",
	})
	err := e.cat.SetDefaults(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid metadata.Namespace")
}

func TestSetDefaults_MetadataWithData(t *testing.T) {
	e := catalogWithMetadata(types.CatalogMeta{
		Namespace:   "test-team2",
		ClusterName: "test-cluster",
	})
	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.cat.EnabledCRDs()["app"]
	assert.Equal(t, entry.CatalogName, "test-cluster-app")
	assert.Equal(t, entry.CatalogNamespace, "test-team2")
}

func TestSetDefaults_MetadataCatalogNameDefaultToClusterName(t *testing.T) {
	e := catalogWithMetadata(types.CatalogMeta{
		Name:        "test",
		Namespace:   "test-team",
		ClusterName: "test-cluster",
	})
	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.cat.EnabledCRDs()["app"]
	assert.Equal(t, entry.CatalogName, "test")
	assert.Equal(t, entry.CatalogNamespace, "test-team")
}

func TestSetDefaults_APITypes(t *testing.T) {
	e := catalogWithAPITypes(types.APITypes{
		Kind:    "Website",
		Group:   "test.inrun.catalog",
		Version: "v1",
	})

	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	// Assert APITypes default
	entry := e.cat.EnabledCRDs()["app"]
	apiPath := entry.APITypes.APIPath
	plural := entry.APITypes.Plural

	assert.Contains(t, apiPath, "/apis")
	assert.Contains(t, plural, "websites")

}

func TestSetDefaults_OperatorBoxFinalizersDefault(t *testing.T) {
	e := catalogWithOperatorBox(&types.OperatorBoxConfig{})
	e.cat.Spec.Finalizers = []string{"spec-finalizer"}

	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.cat.EnabledCRDs()["app"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	if !reflect.DeepEqual(boxFinalizers, e.cat.Spec.Finalizers) {
		t.Fatalf("unexpected error. want true, got false")
	}
}

func TestSetDefaults_SpecFinalizerAddToOperatorBoxFinalizer(t *testing.T) {
	e := catalogWithOperatorBox(&types.OperatorBoxConfig{})
	e.cat.Spec.Finalizers = []string{"spec-finalizer"}

	err := e.cat.SetDefaults(cfg)
	assert.NoError(t, err)

	entry := e.cat.EnabledCRDs()["app"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	boxFinalizers = append(boxFinalizers, "box-finalizer")

	if reflect.DeepEqual(boxFinalizers, e.cat.Spec.Finalizers) {
		t.Fatalf("unexpected error. want false, got true")
	}

	if len(boxFinalizers) != 2 {
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
						Runtime: &types.RuntimeConfig{Finalizers: []string{"target-finalizer"}},
					},
				},
			},
		},
	}

	e := newExec(catalog.NewCatalogForTestWithSpec(
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
		types.CatalogSpec{Finalizers: []string{"spec-finalizer"}},
	))

	entry := e.cat.EnabledCRDs()["myresource"]
	boxFinalizers := entry.OperatorBox.EffectiveFinalizers()
	if reflect.DeepEqual(boxFinalizers, e.cat.Spec.Finalizers) {
		t.Fatalf("unexpected error. want true, got false")
	}

	if len(boxFinalizers) != 2 {
		t.Fatalf("unexpected boxFinalizers length: want 2, got %d", len(boxFinalizers))
	}

	targetFinalizers := entry.Serve.Target.Entries["testfixture"].OperatorBox.EffectiveFinalizers()
	if len(targetFinalizers) != 1 {
		t.Fatalf("unexpected error. want true, got false")
	}
}
