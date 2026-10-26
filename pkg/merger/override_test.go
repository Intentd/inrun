package merger

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const renameBlueprint = "../../examples/migration/kind/rename/blueprint.yaml"

func mergedRename(t *testing.T) *Merger {
	t.Helper()
	m := New(renameBlueprint)
	if err := m.Merge(); err != nil {
		t.Fatalf("merging %s: %v", renameBlueprint, err)
	}
	if _, ok := m.Get("appconfig"); !ok {
		t.Fatal("no appconfig entry in the rename example")
	}
	return m
}

func TestParseSets(t *testing.T) {
	sets, err := ParseSets([]string{"migrate.prune=true", "migrate.selection.selector=tier=web"})
	if err != nil {
		t.Fatal(err)
	}
	if sets[1].Value != "tier=web" {
		t.Errorf("value %q, want everything after the first =", sets[1].Value)
	}
	for _, bad := range []string{"prune", "=true", "migrate..prune=true"} {
		if _, err := ParseSets([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The override changes only what it sets: everything else in the entry, the
// merger's own unserialized fields included, comes back as it was.
func TestOverride_ChangesOnlyThePath(t *testing.T) {
	m := mergedRename(t)
	before, _ := m.Get("appconfig")
	if before.Migrate == nil || before.Migrate.Prune {
		t.Fatal("the rename example should declare a migration with prune off")
	}
	if err := m.Override("appconfig", []Set{{Path: "migrate.prune", Value: "true"}}); err != nil {
		t.Fatal(err)
	}
	after, _ := m.Get("appconfig")
	if !after.Migrate.Prune {
		t.Fatal("migrate.prune not set")
	}
	after.Migrate.Prune = false
	b, _ := yaml.Marshal(before)
	a, _ := yaml.Marshal(after)
	if string(b) != string(a) {
		t.Errorf("the override changed more than migrate.prune:\nbefore:\n%s\nafter:\n%s", b, a)
	}
	if after.Name != before.Name || after.CRDFile != before.CRDFile || after.RegistryRef != before.RegistryRef {
		t.Error("the merger's own fields (not in the YAML) were not kept")
	}
}

func TestOverride_RefusesWhatTheSchemaDoesNotKnow(t *testing.T) {
	for _, tc := range []struct {
		set  Set
		want string
	}{
		{Set{Path: "migrate.prunee", Value: "true"}, "prunee"},
		{Set{Path: "migrate.prune", Value: "[unclosed"}, "not valid YAML"},
		{Set{Path: "migrate.to.crdFile.deeper", Value: "true"}, "not a map"},
	} {
		m := mergedRename(t)
		err := m.Override("appconfig", []Set{tc.set})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tc.set, err, tc.want)
		}
	}
	if err := mergedRename(t).Override("nope", []Set{{Path: "migrate.prune", Value: "true"}}); err == nil {
		t.Error("an unknown entry accepted")
	}
}
