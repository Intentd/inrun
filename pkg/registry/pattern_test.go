package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePatternFile(t *testing.T) {
	in := func(files ...string) func(string) bool {
		set := map[string]bool{}
		for _, f := range files {
			set[f] = true
		}
		return func(rel string) bool { return set[rel] }
	}
	cases := []struct {
		name, file string
		files      []string
		want       string
	}{
		{"root", FileCR, []string{FileCR}, FileCR},
		{"manifests", FileCR, []string{"manifests/cr.yaml"}, "manifests/cr.yaml"},
		{"root wins over the layout", FileCRD, []string{FileCRD, "manifests/crd.yaml"}, FileCRD},
		{"test", FileE2E, []string{"test/e2e.yaml"}, "test/e2e.yaml"},
		{"simulate", FileSimulate, []string{"test/simulate.yaml"}, "test/simulate.yaml"},
		{"no group: root only", FileBlueprint, []string{"manifests/blueprint.yaml"}, ""},
		{"wrong group", FileCR, []string{"test/cr.yaml"}, ""},
		{"missing", FileE2E, nil, ""},
		{"a full path is taken as is", "manifests/cr.yaml", []string{"manifests/cr.yaml"}, "manifests/cr.yaml"},
	}
	for _, c := range cases {
		if got := ResolvePatternFile(c.file, in(c.files...)); got != c.want {
			t.Errorf("%s: ResolvePatternFile(%q, %v) = %q, want %q", c.name, c.file, c.files, got, c.want)
		}
	}
}

func TestFindPatternFile(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{FileBlueprint, "manifests/cr.yaml", "test/e2e.yaml"} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{
		FileBlueprint: FileBlueprint,
		FileCR:        "manifests/cr.yaml",
		FileE2E:       "test/e2e.yaml",
		FileCRD:       "",
		FileSimulate:  "",
	} {
		if got := FindPatternFile(dir, name); got != want {
			t.Errorf("FindPatternFile(%q) = %q, want %q", name, got, want)
		}
	}
}
