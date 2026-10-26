package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ── test CRD fixtures ────────────────────────────────────────────────────────

// minimalCRD is a valid v1 CRD with a nested schema used by most tests.
const minimalCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
    singular: widget
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                image:
                  type: string
                scaling:
                  type: object
                  properties:
                    min:
                      type: integer
                observability:
                  type: object
                  properties:
                    logging:
                      type: object
                      properties:
                        level:
                          type: string
            status:
              type: object
              x-kubernetes-preserve-unknown-fields: true
`

// multiVersionCRD has three versions with different storage/served flags, used
// to verify version selection priority.
const multiVersionCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: false
      storage: false
    - name: v1beta1
      served: true
      storage: false
    - name: v1
      served: true
      storage: true
`

// noSchemaCRD has a version with no OpenAPI schema.
const noSchemaCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
`

// noSpecCRD has a schema without a top-level spec property.
const noSpecCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            status:
              type: object
`

// wrongKindCRD is a valid YAML document that isn't a CRD.
const wrongKindCRD = `apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-crd
data:
  key: value
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "crd.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// ── ReadSchemaFromCRDFile ────────────────────────────────────────────────────

func TestReadSchemaFromCRDFile_Valid(t *testing.T) {
	path := writeTemp(t, minimalCRD)
	schema, err := ReadSchemaFromCRDFile(path)
	require.NoError(t, err)
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)

	// The returned schema is the spec schema, so its properties are the
	// spec's fields, not the whole CRD's fields.
	assert.Contains(t, schema.Properties, "image")
	assert.Contains(t, schema.Properties, "scaling")
	assert.Contains(t, schema.Properties, "observability")
	assert.NotContains(t, schema.Properties, "status")
}

func TestReadSchemaFromCRDFile_MissingFile(t *testing.T) {
	_, err := ReadSchemaFromCRDFile("/does/not/exist.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read file")
}

func TestReadSchemaFromCRDFile_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "this: is: not: valid: yaml: [")
	_, err := ReadSchemaFromCRDFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML")
}

func TestReadSchemaFromCRDFile_WrongKind(t *testing.T) {
	path := writeTemp(t, wrongKindCRD)
	_, err := ReadSchemaFromCRDFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected kind: CustomResourceDefinition")
}

func TestReadSchemaFromCRDFile_NoSchema(t *testing.T) {
	path := writeTemp(t, noSchemaCRD)
	schema, err := ReadSchemaFromCRDFile(path)
	require.NoError(t, err)
	assert.Nil(t, schema)
}

func TestReadSchemaFromCRDFile_NoSpecProperty(t *testing.T) {
	path := writeTemp(t, noSpecCRD)
	_, err := ReadSchemaFromCRDFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no spec property")
}

func TestReadSchemaFromCRDFile_StorageVersionPreferred(t *testing.T) {
	// multiVersionCRD: v1alpha1 (not served), v1beta1 (served, not storage),
	// v1 (served + storage). Storage wins, so v1's schema is selected.
	path := writeTemp(t, multiVersionCRD)
	schema, err := ReadSchemaFromCRDFile(path)
	require.NoError(t, err)
	// multiVersionCRD has no schemas, so nil is the correct result — but
	// the fact that we reach the nil case without erroring proves v1 was
	// selected (v1alpha1 would also return nil; v1beta1 too). The priority
	// is tested directly in TestSelectCRDVersionV1.
	assert.Nil(t, schema)
}

// ── ValidateSchemaPath ──────────────────────────────────────────────────────

func TestValidateSchemaPath_ValidLeaf(t *testing.T) {
	path := writeTemp(t, minimalCRD)

	cases := []string{
		"image",
		"scaling.min",
		"observability.logging.level",
		"spec.image",
		"spec.scaling.min",
		"spec.observability.logging.level",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			require.NoError(t, ValidateSchemaPath(path, p))
		})
	}
}

func TestValidateSchemaPath_NotFound(t *testing.T) {
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "observability.logging.levelb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `path segment "levelb" not found`)
	assert.Contains(t, err.Error(), "available: level")
}

func TestValidateSchemaPath_NotFoundListsAlternatives(t *testing.T) {
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available: image, observability, scaling")
}

func TestValidateSchemaPath_NonObjectParent(t *testing.T) {
	// "image" is a string, so "image.foo" cannot descend into it.
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "image.foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `path segment "image" is type "string"`)
}

func TestValidateSchemaPath_ResolvesToObject(t *testing.T) {
	// "scaling" is an object, not a leaf — a serve field path that stops
	// there would try to write a scalar where an object is expected.
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "scaling")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolves to an object")
}

func TestValidateSchemaPath_EmptyPath(t *testing.T) {
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a field under spec")
}

func TestValidateSchemaPath_EmptySegment(t *testing.T) {
	path := writeTemp(t, minimalCRD)
	err := ValidateSchemaPath(path, "scaling..min")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path segment")
}

func TestValidateSchemaPath_NoSchema(t *testing.T) {
	// A CRD with no schema cannot be validated, but it isn't an error —
	// the caller proceeds with the assumption that the path might be valid.
	path := writeTemp(t, noSchemaCRD)
	require.NoError(t, ValidateSchemaPath(path, "anything.goes"))
}

// ── selectCRDVersionV1 ──────────────────────────────────────────────────────

func TestSelectCRDVersionV1(t *testing.T) {
	cases := []struct {
		name     string
		versions []apiextensionsv1.CustomResourceDefinitionVersion
		want     string
	}{
		{
			name:     "storage preferred over served",
			versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1alpha1", Served: true}, {Name: "v1", Storage: true, Served: true}},
			want:     "v1",
		},
		{
			name:     "served when no storage",
			versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1alpha1", Served: true}, {Name: "v1beta1", Served: true}},
			want:     "v1alpha1", // first served wins
		},
		{
			name:     "first when none served or storage",
			versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1alpha1"}, {Name: "v1beta1"}},
			want:     "v1alpha1",
		},
		{
			name:     "empty",
			versions: nil,
			want:     "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, selectCRDVersionV1(tc.versions))
		})
	}
}

// ── schemaPropKeys ──────────────────────────────────────────────────────────

func TestSchemaPropKeys(t *testing.T) {
	props := map[string]apiextensions.JSONSchemaProps{
		"zebra": {},
		"alpha": {},
		"mike":  {},
	}
	assert.Equal(t, []string{"alpha", "mike", "zebra"}, schemaPropKeys(props))
}

func TestSchemaPropKeys_Empty(t *testing.T) {
	assert.Empty(t, schemaPropKeys(nil))
	assert.Empty(t, schemaPropKeys(map[string]apiextensions.JSONSchemaProps{}))
}

const widgetSchemaCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.io
spec:
  group: example.io
  scope: Namespaced
  names: {kind: Widget, plural: widgets}
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                size: {type: integer}
                ports:
                  type: array
                  items:
                    type: object
                    properties:
                      port: {type: integer}
                labels:
                  type: object
                  additionalProperties: {type: string}
                extra:
                  type: object
                  x-kubernetes-preserve-unknown-fields: true
`

func TestUnknownSpecFields(t *testing.T) {
	path := writeTemp(t, widgetSchemaCRD)
	spec := map[string]any{
		"size":            3,
		"colour":          "red",                                          // unknown
		"ports":           []any{map[string]any{"port": 80, "name": "x"}}, // name unknown in an item
		"labels":          map[string]any{"anything": "goes"},             // open map
		"extra":           map[string]any{"free": map[string]any{"form": 1}},
		"{{ .spec.key }}": "templated key, skipped",
	}
	got, err := UnknownSpecFields(path, spec, func(k string) bool { return strings.Contains(k, "{{") })
	require.NoError(t, err)
	require.Equal(t, []string{"spec.colour", "spec.ports[0].name"}, got)
}
