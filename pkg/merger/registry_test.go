// pkg/merger/registry_test.go
package merger_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/types"
)

// ── RegistryRef ───────────────────────────────────────────────────────────────

func TestRegistryRef_Ref_SHA_Priority(t *testing.T) {
	ref := types.RegistryRef{Branch: "main", Version: "v1.0.0", SHA: "abc123"}
	if ref.Ref() != "abc123" {
		t.Errorf("SHA should take priority, got %q", ref.Ref())
	}
}

func TestRegistryRef_Ref_Version_Priority(t *testing.T) {
	ref := types.RegistryRef{Branch: "main", Version: "v1.0.0"}
	if ref.Ref() != "v1.0.0" {
		t.Errorf("Version should take priority over Branch, got %q", ref.Ref())
	}
}

func TestRegistryRef_Ref_Branch(t *testing.T) {
	ref := types.RegistryRef{Branch: "develop"}
	if ref.Ref() != "develop" {
		t.Errorf("expected develop, got %q", ref.Ref())
	}
}

func TestRegistryRef_Ref_Default(t *testing.T) {
	ref := types.RegistryRef{}
	if ref.Ref() != "main" {
		t.Errorf("default ref should be main, got %q", ref.Ref())
	}
}

func TestRegistryRef_IsDefault(t *testing.T) {
	if !(types.RegistryRef{}).IsDefault() {
		t.Error("empty ref should report IsDefault true")
	}
	if (types.RegistryRef{Branch: "main"}).IsDefault() {
		t.Error("ref with branch set should not be default")
	}
	if (types.RegistryRef{SHA: "abc"}).IsDefault() {
		t.Error("ref with SHA set should not be default")
	}
}

// ── RegistrySource.ResolvedURL ────────────────────────────────────────────────

func TestResolvedURL_AtShorthand(t *testing.T) {
	src := types.RegistrySource{URL: "ghcr.io/intentd/registry/postgres@v14"}
	u, version := src.ResolvedURL()
	if u != "ghcr.io/intentd/registry/postgres" {
		t.Errorf("url: expected stripped @, got %q", u)
	}
	if version != "v14" {
		t.Errorf("version: expected v14, got %q", version)
	}
}

func TestResolvedURL_AtShorthand_GitURL(t *testing.T) {
	src := types.RegistrySource{URL: "https://github.com/myorg/registry@main"}
	u, version := src.ResolvedURL()
	if u != "https://github.com/myorg/registry" {
		t.Errorf("url: expected %q, got %q", "https://github.com/myorg/registry", u)
	}
	if version != "main" {
		t.Errorf("version: expected main, got %q", version)
	}
}

func TestResolvedURL_ExplicitVersion(t *testing.T) {
	src := types.RegistrySource{URL: "ghcr.io/intentd/registry/postgres", Version: "v14.2.0"}
	u, version := src.ResolvedURL()
	if u != "ghcr.io/intentd/registry/postgres" {
		t.Errorf("url: expected unchanged, got %q", u)
	}
	if version != "v14.2.0" {
		t.Errorf("version: expected v14.2.0, got %q", version)
	}
}

func TestResolvedURL_AtShorthand_Takes_Priority(t *testing.T) {
	src := types.RegistrySource{URL: "ghcr.io/intentd/registry/postgres@v14", Version: "v15"}
	_, version := src.ResolvedURL()
	if version != "v14" {
		t.Errorf("@ shorthand should take priority, got version %q", version)
	}
}

func TestResolvedURL_DefaultVersion_OCI(t *testing.T) {
	src := types.RegistrySource{URL: "ghcr.io/intentd/registry/postgres", OCI: true}
	_, version := src.ResolvedURL()
	if version != "latest" {
		t.Errorf("OCI default version should be 'latest', got %q", version)
	}
}

func TestResolvedURL_DefaultVersion_Git(t *testing.T) {
	src := types.RegistrySource{URL: "https://github.com/myorg/registry", OCI: false}
	_, version := src.ResolvedURL()
	if version != "main" {
		t.Errorf("Git default version should be 'main', got %q", version)
	}
}

// ── RegistrySource.SourceFile ─────────────────────────────────────────────────

func TestSourceFile_Default_IsCatalog(t *testing.T) {
	src := types.RegistrySource{URL: "https://github.com/myorg/r"}
	if src.SourceFile() != "catalog.yaml" {
		t.Errorf("default source file should be catalog.yaml, got %q", src.SourceFile())
	}
}

func TestSourceFile_UseStack_IsStack(t *testing.T) {
	src := types.RegistrySource{UseStack: true}
	if src.SourceFile() != "stack.yaml" {
		t.Errorf("UseStack=true should return stack.yaml, got %q", src.SourceFile())
	}
}

// ── validatePatternStructure ──────────────────────────────────────────────────

// Catalog pattern: only catalog.yaml is required.
func TestValidatePatternStructure_CatalogOnly_NoError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "catalog.yaml", "kind: Catalog\n")
	if err := merger.ExportedValidatePatternStructure(dir, "test-url", "v1.0.0"); err != nil {
		t.Errorf("catalog.yaml alone should be sufficient: %v", err)
	}
}

// Catalog pattern with all optional files present.
func TestValidatePatternStructure_CatalogWithOptionals_NoError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "catalog.yaml", "kind: Catalog\n")
	writeFile(t, dir, "crd.yaml", "content")
	writeFile(t, dir, "cr.yaml", "content")
	writeFile(t, dir, "README.md", "content")
	if err := merger.ExportedValidatePatternStructure(dir, "test-url", "v1.0.0"); err != nil {
		t.Errorf("catalog with all optional files should pass: %v", err)
	}
}

// Module pattern: only module.yaml is required.
func TestValidatePatternStructure_ModuleOnly_NoError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "module.yaml", "kind: Module\n")
	if err := merger.ExportedValidatePatternStructure(dir, "test-url", "v1.0.0"); err != nil {
		t.Errorf("module.yaml alone should be sufficient: %v", err)
	}
}

// No recognised pattern file → error.
func TestValidatePatternStructure_NoPatternFile_Error(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "crd.yaml", "content")
	writeFile(t, dir, "README.md", "content")
	if err := merger.ExportedValidatePatternStructure(dir, "test-url", "v1.0.0"); err == nil {
		t.Fatal("expected error when no catalog.yaml or module.yaml present")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// ── GitHub raw URL construction ───────────────────────────────────────────────

func TestGitHubRawURL_Construction(t *testing.T) {
	got := merger.ExportedGitHubRawURL("https://github.com/myorg/myrepo", "main", "catalog.yaml")
	want := "https://raw.githubusercontent.com/myorg/myrepo/main/catalog.yaml"
	if got != want {
		t.Errorf("githubRawURL: got %q, want %q", got, want)
	}
}

// patternServer serves the 5 required pattern files at /<version>/<filename>.
func patternServer(t *testing.T, version string, overrides map[string][]byte) *httptest.Server {
	t.Helper()
	files := map[string][]byte{
		"/" + version + "/crd.yaml":     []byte("kind: CustomResourceDefinition"),
		"/" + version + "/catalog.yaml": testCatalogYAML,
		"/" + version + "/stack.yaml":   []byte("apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: c\nspec:\n  crds:\n    - name: myapp\n      enabled: true\n      apiTypes:\n        group: test.inrun.dev\n        version: v1alpha1\n        kind: MyApp\n        plural: myapps\n"),
		"/" + version + "/cr.yaml":      []byte("kind: MyApp"),
		"/" + version + "/README.md":    []byte("# MyApp"),
	}
	for k, v := range overrides {
		files[k] = v
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
}

var testCatalogYAML = []byte(`apiVersion: inrun.dev/v1
kind: Catalog
metadata:
  name: test-catalog
spec:
  crds:
    - name: myapp
      enabled: true
      apiTypes:
        group: test.inrun.dev
        version: v1alpha1
        kind: MyApp
        plural: myapps
      operatorBox:
        default: true
`)

// NOTE: Tests that require network access (git clone, OCI pull, real GitHub/GitLab)
// belong in tests/integration/stack/ behind the integration build tag.
