package structured

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMissingPaths_Equal(t *testing.T) {
	spec := map[string]interface{}{"a": int64(1), "b": map[string]interface{}{"c": "x"}}
	assert.Empty(t, MissingPaths(spec, spec))
}

func TestMissingPaths_ServerDefaultsIgnored(t *testing.T) {
	want := map[string]interface{}{"a": 1}
	got := map[string]interface{}{"a": float64(1), "defaulted": true, "nested": map[string]interface{}{"x": 1}}
	assert.Empty(t, MissingPaths(want, got))
}

func TestMissingPaths_DroppedAndChanged(t *testing.T) {
	want := map[string]interface{}{
		"keep":    "v",
		"dropped": "v",
		"nested":  map[string]interface{}{"changed": "old", "same": 1},
		"list":    []interface{}{"a", "b"},
	}
	got := map[string]interface{}{
		"keep":   "v",
		"nested": map[string]interface{}{"changed": "new", "same": 1},
		"list":   []interface{}{"a"},
	}
	assert.Equal(t, []string{"dropped", "list", "nested.changed"}, MissingPaths(want, got))
}

func TestMissingPaths_NilWantIsNotMissing(t *testing.T) {
	assert.Empty(t, MissingPaths(map[string]interface{}{"gone": nil}, map[string]interface{}{}))
}

func TestMissingPaths_TypeChange(t *testing.T) {
	want := map[string]interface{}{"a": map[string]interface{}{"b": 1}}
	got := map[string]interface{}{"a": "flat"}
	assert.Equal(t, []string{"a"}, MissingPaths(want, got))
}

func TestMissingPaths_NilGot(t *testing.T) {
	assert.Equal(t, []string{"a"}, MissingPaths(map[string]interface{}{"a": 1}, nil))
}

func TestMissingPaths_NilWant(t *testing.T) {
	assert.Empty(t, MissingPaths(nil, map[string]interface{}{"a": 1}))
}
