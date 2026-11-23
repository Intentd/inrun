package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kome/pkg/declaration"
	"github.com/intentd/tools/fileio"
)

func TestDeclarationFor_Field(t *testing.T) {
	crd := &types.CRDEntry{
		APITypes:         types.APITypes{Group: "example.com", Version: "v1", Kind: "Website", Plural: "websites"},
		GroupVersionKind: schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Website"},
		ResolvedCRDFile:  "/bp/website-crd.yaml",
		Serve: &types.ServeConfig{Enabled: true, Fields: map[string]types.ServeFieldConfig{
			"replicas": {Path: "scaling.min", Migrate: &types.ServeFieldMigrate{From: "spec.replicas"}},
			"schedule": {Values: map[string]string{"schedule.minute": "{{ .value }}"}, Migrate: &types.ServeFieldMigrate{From: "schedule"}},
		}},
		Migrate: &types.CRMigrate{
			From:         &types.CRMigrateSource{ResolvedCRDFile: "/bp/website-old.yaml", ResolvedCRFiles: []string{"/bp/old.yaml"}},
			Transitional: &types.CRMigrateTransitional{ResolvedCRDFile: "/bp/website-transitional.yaml"},
			To:           types.CRMigrateTarget{ResolvedCRFiles: []string{"/bp/new.yaml"}},
		},
	}
	d := declarationFor(crd)
	assert.Equal(t, declaration.LevelField, d.Level())
	assert.Equal(t, "/bp/website-old.yaml", d.From.Schema)
	assert.Equal(t, []string{"/bp/old.yaml"}, d.From.Objects)
	assert.Equal(t, "/bp/website-crd.yaml", d.To.Schema, "the CRD's crdFile is the new schema")
	assert.Equal(t, []string{"/bp/new.yaml"}, d.To.Objects, "to.crFiles do not make it kind level")
	assert.Equal(t, "/bp/website-transitional.yaml", d.Transitional.Schema)
	assert.Equal(t, declaration.Field{From: "replicas", To: "scaling.min"}, d.Spec["replicas"])
	assert.Equal(t, []string{"schedule.minute"}, d.Spec["schedule"].Destinations("schedule"))
}

func TestDeclarationFor_Kind(t *testing.T) {
	crd := &types.CRDEntry{
		APITypes:         types.APITypes{Group: "old.io", Version: "v1", Kind: "App", Plural: "apps"},
		GroupVersionKind: schema.GroupVersionKind{Group: "old.io", Version: "v1", Kind: "App"},
		ResolvedCRDFile:  "/bp/app-crd.yaml",
		Migrate: &types.CRMigrate{
			From: &types.CRMigrateSource{ResolvedCRFiles: []string{"/bp/apps.yaml"}},
			To: types.CRMigrateTarget{APIVersion: "new.io/v1", Kind: "App", ResolvedCRDFile: "/bp/new-crd.yaml",
				CopyStatus: true, ResolvedCRFiles: []string{"/bp/existing.yaml"}},
			Lifecycle: &types.MigrateLifecycle{Prune: true, Children: "orphan", Finalizers: []string{"example.io/cleanup"}},
			Needs:     []string{"website"},
		},
	}
	d := declarationFor(crd)
	assert.Equal(t, declaration.LevelKind, d.Level())
	assert.Equal(t, "/bp/app-crd.yaml", d.From.Schema, "the CRD's crdFile is the source")
	assert.Equal(t, []string{"/bp/apps.yaml"}, d.From.Objects)
	assert.Equal(t, "/bp/new-crd.yaml", d.To.Schema)
	assert.Equal(t, []string{"/bp/existing.yaml"}, d.To.Objects)
	assert.True(t, d.To.CopyStatus)
	assert.Equal(t, &declaration.Lifecycle{Prune: true, Children: "orphan", Finalizers: []string{"example.io/cleanup"}}, d.Lifecycle)
	assert.Equal(t, []string{"website"}, d.Needs)
}

func TestAppliedCRDFile(t *testing.T) {
	crd := &types.CRDEntry{CRDFile: "crd.yaml"}
	assert.Equal(t, "crd.yaml", crd.AppliedCRDFile())
	crd.Migrate = &types.CRMigrate{Transitional: &types.CRMigrateTransitional{CRDFile: "transitional.yaml"}}
	assert.Equal(t, "transitional.yaml", crd.AppliedCRDFile(), "a declared transitional CRD is applied in crdFile's place")
}

const relocation = "../../examples/migration/field/relocation"

func build(t *testing.T, path string) *blueprint.Blueprint {
	t.Helper()
	m := merger.New(path)
	require.NoError(t, m.Merge())
	bp, err := pipeline.BuildExpanded(config.NewDefaultConfig(), m)
	require.NoError(t, err)
	return bp
}

// relocationWithout copies the relocation example's Blueprint, its manifests
// and its serve include into a temp dir, with its transitional left
// out, and returns the copy's Blueprint path.
func relocationWithout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{"manifests/old-crd.yaml", "manifests/crd-final.yaml", "manifests/old-crs.yaml", "manifests/new-crs.yaml", "serve/appconfig.yaml"} {
		data, err := fileio.ReadLocal(filepath.Join(relocation, rel))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, rel), data, 0o600))
	}
	data, err := fileio.ReadLocal(filepath.Join(relocation, "blueprint.yaml"))
	require.NoError(t, err)
	declared := "        transitional:\n          crdFile: ./manifests/crd-transitional.yaml\n"
	require.Contains(t, string(data), declared)
	path := filepath.Join(dir, "blueprint.yaml")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), declared, "", 1)), 0o600))
	return path
}

func TestTransitional_SkipsADeclaredOne(t *testing.T) {
	_, ok, err := Transitional(build(t, filepath.Join(relocation, "blueprint.yaml")), "appconfig")
	require.NoError(t, err)
	assert.False(t, ok, "a declared transitional CRD is applied from its file")
}

func TestTransitional_GeneratesTheUndeclared(t *testing.T) {
	out, ok, err := Transitional(build(t, relocationWithout(t)), "appconfig")
	require.NoError(t, err)
	require.True(t, ok)

	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(out, &crd))
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	for _, p := range []string{"replicas", "logLevel", "scaling", "observability"} {
		assert.Contains(t, spec.Properties, p, "old and new paths are both declared")
	}
	var cols []string
	for _, c := range crd.Spec.Versions[0].AdditionalPrinterColumns {
		cols = append(cols, c.JSONPath)
	}
	assert.Contains(t, cols, ".spec.replicas", "an unmigrated CR still shows its old value")
}
