package core

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSelection(t *testing.T) {
	declared := func(s types.MigrateSelection) *types.CRDEntry {
		return &types.CRDEntry{Migrate: &types.CRMigrate{Selection: s}}
	}
	cases := []struct {
		name    string
		cli     Runner
		crd     *types.CRDEntry
		want    Selection
		wantErr string
	}{
		{name: "cli only, field level", cli: Runner{Namespaces: []string{"a"}, Selector: "x=1"}, crd: &types.CRDEntry{},
			want: Selection{Namespaces: []string{"a"}, Selector: "x=1"}},
		{name: "declared only", crd: declared(types.MigrateSelection{Namespaces: []string{"a", "b"}, Name: "n", Selector: "x=1"}),
			want: Selection{Namespaces: []string{"a", "b"}, Name: "n", Selector: "x=1"}},
		{name: "cli narrows namespaces", cli: Runner{Namespaces: []string{"b", "c"}}, crd: declared(types.MigrateSelection{Namespaces: []string{"a", "b"}}),
			want: Selection{Namespaces: []string{"b"}}},
		{name: "cli outside declared namespaces", cli: Runner{Namespaces: []string{"c"}}, crd: declared(types.MigrateSelection{Namespaces: []string{"a", "b"}}),
			wantErr: "outside migrate.selection.namespaces"},
		{name: "names agree", cli: Runner{Name: "n"}, crd: declared(types.MigrateSelection{Name: "n"}),
			want: Selection{Name: "n"}},
		{name: "names conflict", cli: Runner{Name: "m"}, crd: declared(types.MigrateSelection{Name: "n"}),
			wantErr: "cannot both apply"},
		{name: "selectors AND", cli: Runner{Selector: "a=1"}, crd: declared(types.MigrateSelection{Selector: "b in (x,y)"}),
			want: Selection{Selector: "a=1,b in (x,y)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cli.Selection(tc.crd)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSelectionIncludes(t *testing.T) {
	sel := Selection{Namespaces: []string{"a"}, Name: "app", Selector: "team=x"}
	lbls := map[string]string{"team": "x"}

	ok, err := sel.Includes("app", "a", lbls)
	require.NoError(t, err)
	assert.True(t, ok)

	for _, c := range []struct{ name, ns string }{{"other", "a"}, {"app", "b"}} {
		ok, err := sel.Includes(c.name, c.ns, lbls)
		require.NoError(t, err)
		assert.False(t, ok, c)
	}

	ok, err = sel.Includes("app", "a", map[string]string{"team": "y"})
	require.NoError(t, err)
	assert.False(t, ok)

	// A cluster-scoped source (no namespace) is not filtered by namespaces.
	ok, err = sel.Includes("app", "", lbls)
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = Selection{Selector: "=bad"}.Includes("app", "a", nil)
	assert.Error(t, err)
}

func TestSelectionIsEmptyEqual(t *testing.T) {
	assert.True(t, Selection{}.IsEmpty())
	assert.False(t, Selection{Name: "x"}.IsEmpty())
	assert.True(t, Selection{Namespaces: []string{"a"}}.Equal(Selection{Namespaces: []string{"a"}}))
	assert.False(t, Selection{Namespaces: []string{"a"}}.Equal(Selection{Namespaces: []string{"b"}}))
}

func TestAdmits(t *testing.T) {
	cr := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"env": "prod", "tier": "gold"},
	}}
	crd := func(sel types.MigrateSelection) *types.CRDEntry {
		return &types.CRDEntry{Migrate: &types.CRMigrate{Selection: sel}}
	}
	r := &Runner{}

	assert.True(t, r.Admits(&types.CRDEntry{}, cr), "no migrate block")
	assert.True(t, r.Admits(crd(types.MigrateSelection{Selector: "x=y"}), cr), "no conditions")
	assert.True(t, r.Admits(crd(types.MigrateSelection{When: []types.Condition{{Field: "spec.env", Equals: "prod"}}}), cr))
	assert.False(t, r.Admits(crd(types.MigrateSelection{When: []types.Condition{{Field: "spec.env", Equals: "dev"}}}), cr))
	assert.True(t, r.Admits(crd(types.MigrateSelection{Or: []types.Condition{
		{Field: "spec.env", Equals: "dev"}, {Field: "spec.tier", Equals: "gold"},
	}}), cr))
}

func TestSelection_FieldLevelUsesDeclared(t *testing.T) {
	// A migrate block without migrate.to still scopes the run.
	crd := &types.CRDEntry{Migrate: &types.CRMigrate{Selection: types.MigrateSelection{Selector: "tier=gold"}}}
	got, err := (&Runner{Selector: "app=x"}).Selection(crd)
	require.NoError(t, err)
	assert.Equal(t, "app=x,tier=gold", got.Selector)
}
