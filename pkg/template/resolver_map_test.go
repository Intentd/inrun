package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type labelMap map[string]string

func mapResolver() *Resolver {
	return NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "app"},
		"spec":     map[string]interface{}{"replicas": 3, "enabled": true},
	})
}

func TestResolveNested(t *testing.T) {
	in := map[string]any{
		"name":     "{{ .metadata.name }}",
		"replicas": "{{ .spec.replicas }}",
		"enabled":  "{{ .spec.enabled }}",
		"plain":    "42",
		"number":   7,
		"nested":   map[string]any{"who": "{{ .metadata.name }}-x"},
		"list":     []any{"{{ .spec.replicas }}", "b", 1},
		"labels":   map[string]string{"app": "{{ .metadata.name }}", "n": "{{ .spec.replicas }}"},
		"named":    labelMap{"app": "{{ .metadata.name }}"},
	}
	out, err := mapResolver().ResolveNested(in)
	require.NoError(t, err)

	assert.Equal(t, "app", out["name"])
	assert.Equal(t, float64(3), out["replicas"], "resolved templates are coerced (JSON number)")
	assert.Equal(t, true, out["enabled"])
	assert.Equal(t, "42", out["plain"], "plain strings are not coerced")
	assert.Equal(t, 7, out["number"])
	assert.Equal(t, map[string]any{"who": "app-x"}, out["nested"])
	assert.Equal(t, []any{float64(3), "b", 1}, out["list"])
	assert.Equal(t, map[string]any{"app": "app", "n": "3"}, out["labels"], "string-map values are not coerced")
	assert.Equal(t, map[string]any{"app": "app"}, out["named"])

	assert.Equal(t, "{{ .metadata.name }}", in["name"], "input is not modified")
}

func TestResolveNested_ErrorNamesKey(t *testing.T) {
	_, err := mapResolver().ResolveNested(map[string]any{
		"outer": map[string]any{"bad": "{{ .metadata.name }"},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "outer: bad:")
}

func TestResolveNestedLenient_KeepsFailures(t *testing.T) {
	out := mapResolver().ResolveNestedLenient(map[string]any{
		"bad":    "{{ .metadata.name }",
		"good":   "{{ .metadata.name }}",
		"labels": map[string]string{"bad": "{{ nope"},
	})
	assert.Equal(t, "{{ .metadata.name }", out["bad"])
	assert.Equal(t, "app", out["good"])
	assert.Equal(t, map[string]any{"bad": "{{ nope"}, out["labels"])
}
