//go:build !runtime && !gateway

package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/types"
)

func TestReadIntentFile_YAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intent.yaml")
	if err := os.WriteFile(path, []byte("target: app\nname: foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := readIntentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw["target"] != "app" || raw["name"] != "foo" {
		t.Errorf("got %+v", raw)
	}
}

func TestReadIntentFile_JSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(path, []byte(`{"target":"app","name":"foo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := readIntentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw["target"] != "app" || raw["name"] != "foo" {
		t.Errorf("got %+v", raw)
	}
}

func TestReadIntentFile_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readIntentFile(path); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestReadIntentFile_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "intent.yaml")
	if err := os.WriteFile(path, []byte("key: [1, 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readIntentFile(path); err == nil {
		t.Fatal("expected an error for invalid YAML")
	}
}

func TestReadIntentFile_MissingFile(t *testing.T) {
	if _, err := readIntentFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestResolveDefaultIntentFile_PrefersYAML(t *testing.T) {
	chdirTemp(t)
	writeFile(t, "intent.yaml", "target: app\n")
	writeFile(t, "intent.json", `{"target":"app"}`)

	if got := resolveDefaultIntentFile(); got != "intent.yaml" {
		t.Errorf("got %q, want intent.yaml", got)
	}
}

func TestResolveDefaultIntentFile_FallsBackToJSON(t *testing.T) {
	chdirTemp(t)
	writeFile(t, "intent.json", `{"target":"app"}`)

	if got := resolveDefaultIntentFile(); got != "intent.json" {
		t.Errorf("got %q, want intent.json", got)
	}
}

func TestResolveDefaultIntentFile_None(t *testing.T) {
	chdirTemp(t)

	if got := resolveDefaultIntentFile(); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestNamespaceOrAny(t *testing.T) {
	if got := namespaceOrAny(""); got != "(all namespaces)" {
		t.Errorf("got %q", got)
	}
	if got := namespaceOrAny("prod"); got != "prod" {
		t.Errorf("got %q", got)
	}
}

func TestPayloadKeys_SortsAlphabetically(t *testing.T) {
	got := cmdutil.SortedKeys(map[string]string{"zeta": "1", "alpha": "2", "mid": "3"})
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestPlayRead_UnknownTarget(t *testing.T) {
	cat := chainTestCatalog("", nil)
	if err := playRead(cat, "does-not-exist", "dev", types.ServeOpGet, "default", "x"); err == nil {
		t.Fatal("expected an error for an unknown target")
	}
}

func TestPlayRead_MissingTargetFlag(t *testing.T) {
	cat := chainTestCatalog("", nil)
	if err := playRead(cat, "", "dev", types.ServeOpGet, "default", "x"); err == nil {
		t.Fatal("expected an error when --target is empty")
	}
}

func TestPlayRead_MissingNameForGet(t *testing.T) {
	cat := chainTestCatalog("", nil)
	if err := playRead(cat, "servicerequest", "dev", types.ServeOpGet, "default", ""); err == nil {
		t.Fatal("expected an error when --name is empty for get")
	}
}

func TestPlayRead_ListDoesNotRequireName(t *testing.T) {
	cat := chainTestCatalog("", nil)
	if err := playRead(cat, "servicerequest", "dev", types.ServeOpList, "default", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlayRead_TokenDenied(t *testing.T) {
	cat := chainTestCatalog("", map[string]types.ServeTokenPermissions{
		"dev": {Permissions: types.ServePermissionSet{Global: []string{"create"}}}, // no get
	})
	if err := playRead(cat, "servicerequest", "dev", types.ServeOpGet, "default", "x"); err == nil {
		t.Fatal("expected the token to be denied get")
	}
}

func TestPlayRead_AllowedGet(t *testing.T) {
	cat := chainTestCatalog("", nil) // no token restrictions -> allow all
	if err := playRead(cat, "servicerequest", "dev", types.ServeOpGet, "default", "x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlayRead_Delete(t *testing.T) {
	cat := chainTestCatalog("", nil)
	if err := playRead(cat, "servicerequest", "dev", types.ServeOpDelete, "default", "x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// chdirTemp switches the working directory to a fresh temp dir for the
// duration of the test, restoring the original directory on cleanup.
func chdirTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
