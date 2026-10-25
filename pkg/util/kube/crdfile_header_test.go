package kube

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCRDFileHeader_Valid(t *testing.T) {
	h, err := ReadCRDFileHeader(writeTemp(t, minimalCRD))
	require.NoError(t, err)
	assert.Equal(t, "example.com", h.Spec.Group)
	assert.Equal(t, "Widget", h.Spec.Names.Kind)
	assert.Equal(t, "widgets", h.Spec.Names.Plural)
	assert.Equal(t, "v1", h.Version())
	assert.Equal(t, "example.com/v1", h.APIVersion())
	ns, err := h.Namespaced()
	require.NoError(t, err)
	assert.True(t, ns)
}

func TestCRDFileHeader_Version(t *testing.T) {
	cases := map[string]struct {
		versions []CRDFileVersion
		want     string
	}{
		"storage wins":           {[]CRDFileVersion{{Name: "a", Served: true}, {Name: "b", Storage: true}}, "b"},
		"served when no storage": {[]CRDFileVersion{{Name: "a"}, {Name: "b", Served: true}}, "b"},
		"first otherwise":        {[]CRDFileVersion{{Name: "a"}, {Name: "b"}}, "a"},
		"none":                   {nil, ""},
	}
	for name, tc := range cases {
		h := &CRDFileHeader{}
		h.Spec.Versions = tc.versions
		assert.Equal(t, tc.want, h.Version(), name)
	}
}

func TestReadCRDFileHeader_MultiVersionPicksStorage(t *testing.T) {
	h, err := ReadCRDFileHeader(writeTemp(t, multiVersionCRD))
	require.NoError(t, err)
	assert.Equal(t, "v1", h.Version())
}

func TestCRDFileHeader_Namespaced(t *testing.T) {
	for scope, want := range map[string]bool{"": true, "Namespaced": true, "Cluster": false} {
		h := &CRDFileHeader{}
		h.Spec.Scope = scope
		got, err := h.Namespaced()
		require.NoError(t, err, scope)
		assert.Equal(t, want, got, scope)
	}
	h := &CRDFileHeader{}
	h.Spec.Scope = "Global"
	_, err := h.Namespaced()
	assert.ErrorContains(t, err, "must be Namespaced or Cluster")
}

func TestReadCRDFileHeader_Errors(t *testing.T) {
	cases := map[string]struct {
		edit func(string) string
		want string
	}{
		"wrong kind":    {func(s string) string { return wrongKindCRD }, "expected kind: CustomResourceDefinition"},
		"no group":      {func(s string) string { return strings.Replace(s, "  group: example.com\n", "", 1) }, "spec.group is missing"},
		"no names.kind": {func(s string) string { return strings.Replace(s, "    kind: Widget\n", "", 1) }, "spec.names.kind is missing"},
		"no plural":     {func(s string) string { return strings.Replace(s, "    plural: widgets\n", "", 1) }, "spec.names.plural is missing"},
		"bad scope":     {func(s string) string { return strings.Replace(s, "scope: Namespaced", "scope: Global", 1) }, "spec.scope"},
		"invalid yaml":  {func(s string) string { return "spec: [" }, "invalid YAML"},
		"no versions": {func(s string) string {
			return s[:strings.Index(s, "  versions:")]
		}, "spec.versions is empty"},
	}
	for name, tc := range cases {
		_, err := ReadCRDFileHeader(writeTemp(t, tc.edit(minimalCRD)))
		require.Error(t, err, name)
		assert.ErrorContains(t, err, tc.want, name)
	}
}

func TestReadCRDFileHeader_MissingFile(t *testing.T) {
	_, err := ReadCRDFileHeader("/does/not/exist.yaml")
	assert.ErrorContains(t, err, "cannot read file")
}
