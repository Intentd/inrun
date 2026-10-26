package blueprint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

const newConfigCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: newconfigs.next.io
spec:
  group: next.io
  scope: Namespaced
  names:
    kind: NewConfig
    plural: newconfigs
  versions:
    - name: v2
      served: true
      storage: true
`

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPopulateMigrateTarget_FillsFromFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "new.yaml", newConfigCRD)
	entry := &types.CRDEntry{Migrate: &types.CRMigrate{To: types.CRMigrateTarget{CRDFile: "new.yaml"}}}

	if err := blueprint.PopulateMigrateTargetFromCRDFile(entry, dir); err != nil {
		t.Fatal(err)
	}
	to := entry.Migrate.To
	if to.APIVersion != "next.io/v2" || to.Kind != "NewConfig" || to.Plural != "newconfigs" {
		t.Errorf("got %s %s %s", to.APIVersion, to.Kind, to.Plural)
	}
	if to.ResolvedCRDFile != filepath.Join(dir, "new.yaml") {
		t.Errorf("ResolvedCRDFile = %q", to.ResolvedCRDFile)
	}
}

func TestPopulateMigrateTarget_MatchingDeclarationOK(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "new.yaml", newConfigCRD)
	entry := &types.CRDEntry{Migrate: &types.CRMigrate{To: types.CRMigrateTarget{
		CRDFile: "new.yaml", APIVersion: "next.io/v2", Kind: "NewConfig",
	}}}
	if err := blueprint.PopulateMigrateTargetFromCRDFile(entry, dir); err != nil {
		t.Fatal(err)
	}
}

func TestPopulateMigrateTarget_ConflictIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "new.yaml", newConfigCRD)
	for field, to := range map[string]types.CRMigrateTarget{
		"apiVersion": {CRDFile: "new.yaml", APIVersion: "next.io/v1"},
		"kind":       {CRDFile: "new.yaml", Kind: "Other"},
		"plural":     {CRDFile: "new.yaml", Plural: "others"},
	} {
		entry := &types.CRDEntry{Migrate: &types.CRMigrate{To: to}}
		err := blueprint.PopulateMigrateTargetFromCRDFile(entry, dir)
		if err == nil || !strings.Contains(err.Error(), "migrate.to."+field) {
			t.Errorf("%s: err = %v", field, err)
		}
	}
}

func TestPopulateMigrateTarget_RemoteAndAbsentSkipped(t *testing.T) {
	for _, entry := range []*types.CRDEntry{
		{},
		{Migrate: &types.CRMigrate{}},
		{Migrate: &types.CRMigrate{To: types.CRMigrateTarget{CRDFile: "https://example.com/crd.yaml"}}},
	} {
		if err := blueprint.PopulateMigrateTargetFromCRDFile(entry, t.TempDir()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestResolveCRDFiles_InlinesMigrateTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "new.yaml", newConfigCRD)
	bp := writeFile(t, dir, "blueprint.yaml", `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test
spec:
  crds:
    old:
      apiTypes:
        group: old.io
        version: v1
        kind: OldConfig
        plural: oldconfigs
      migrate:
        to:
          crdFile: new.yaml
`)

	out, err := blueprint.ResolveCRDFiles(bp)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Spec struct {
			CRDs map[string]struct {
				Migrate struct {
					To map[string]interface{} `yaml:"to"`
				} `yaml:"migrate"`
			} `yaml:"crds"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	to := doc.Spec.CRDs["old"].Migrate.To
	if to["apiVersion"] != "next.io/v2" || to["kind"] != "NewConfig" || to["plural"] != "newconfigs" {
		t.Errorf("migrate.to = %v", to)
	}
	if _, ok := to["crdFile"]; ok {
		t.Error("crdFile must be removed")
	}
}
