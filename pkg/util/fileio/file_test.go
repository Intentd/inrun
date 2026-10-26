package fileio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if FileExists(f) {
		t.Fatalf("FileExists(%q) = true before creation", f)
	}
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !FileExists(f) {
		t.Fatalf("FileExists(%q) = false after creation", f)
	}
	if FileExists(filepath.Join(dir, "missing")) {
		t.Fatal("FileExists reported a missing file as existing")
	}
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if !IsDir(sub) {
		t.Fatalf("IsDir(%q) = false, want true", sub)
	}
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if IsDir(f) {
		t.Fatalf("IsDir(%q) = true for a file", f)
	}
	if IsDir(filepath.Join(dir, "missing")) {
		t.Fatal("IsDir reported a missing path as a dir")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("dst contents = %q, want %q", got, "hello")
	}
	if err := CopyFile(filepath.Join(dir, "nope"), dst); err == nil {
		t.Fatal("CopyFile with missing src: want error, got nil")
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "dst")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	for rel, want := range map[string]string{
		"a.txt":        "a",
		"nested/b.txt": "b",
	} {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", rel, got, want)
		}
	}
}

func TestJoinRelative(t *testing.T) {
	cases := []struct {
		name, base, p, want string
	}{
		{"relative", "/a/b/blueprint.yaml", "manifests/crd.yaml", "/a/b/manifests/crd.yaml"},
		{"absolute", "/a/b/blueprint.yaml", "/abs/crd.yaml", "/abs/crd.yaml"},
		{"dot-slash", "/a/b/blueprint.yaml", "./crd.yaml", "/a/b/crd.yaml"},
		{"parent", "/a/b/blueprint.yaml", "../shared/crd.yaml", "/a/shared/crd.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JoinRelative(tc.base, tc.p); got != tc.want {
				t.Fatalf("JoinRelative(%q, %q) = %q, want %q", tc.base, tc.p, got, tc.want)
			}
		})
	}
}

func TestReadLocal(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLocal(f)
	if err != nil {
		t.Fatalf("ReadLocal: %v", err)
	}
	if string(got) != "data" {
		t.Fatalf("ReadLocal = %q, want %q", got, "data")
	}
	if _, err := ReadLocal(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("ReadLocal on missing file: want error, got nil")
	}
}

func TestPathOrBaseMatches(t *testing.T) {
	cases := []struct {
		name, pattern, path string
		want                bool
	}{
		{"glob base", "*.yaml", "/a/b/c.yaml", true},
		{"glob base no match", "*.json", "/a/b/c.yaml", false},
		{"exact component", "testdata", "/a/testdata/c.yaml", true},
		{"exact base", "c.yaml", "/a/b/c.yaml", true},
		{"component deeper", "vendor", "/a/vendor/x/y.yaml", true},
		{"no match", "nope", "/a/b/c.yaml", false},
		{"slash pattern never matches base", "a/b", "/a/b/c.yaml", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PathOrBaseMatches(tc.pattern, tc.path); got != tc.want {
				t.Fatalf("PathOrBaseMatches(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

// writeTree creates files under dir from a path→contents map.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, contents := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoverFiles(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a/e2e.yaml":        "a",
		"b/e2e.yaml":        "b",
		"b/simulate.yaml":   "b-sim",
		"c/skipme/e2e.yaml": "c",
		"d/aggregator.yaml": "agg",
		"e/not-an-e2e.yaml": "n",
	})

	got, err := DiscoverFiles(dir, "e2e.yaml", []string{"skipme"}, func(p string) bool {
		return filepath.Base(p) == "aggregator.yaml"
	})
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	want := []string{
		filepath.Join(dir, "a", "e2e.yaml"),
		filepath.Join(dir, "b", "e2e.yaml"),
		filepath.Join(dir, "e", "not-an-e2e.yaml"), // suffix match, not name match
	}
	if len(got) != len(want) {
		t.Fatalf("DiscoverFiles returned %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDiscoverFilesExcludeAndSort(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"z/e2e.yaml": "z",
		"a/e2e.yaml": "a",
		"m/e2e.yaml": "m",
	})

	got, err := DiscoverFiles(dir, "e2e.yaml", nil, func(p string) bool {
		return filepath.Base(filepath.Dir(p)) == "m"
	})
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d paths, want 2: %v", len(got), got)
	}
	if filepath.Base(filepath.Dir(got[0])) != "a" || filepath.Base(filepath.Dir(got[1])) != "z" {
		t.Fatalf("results not sorted by path: %v", got)
	}
}

func TestDiscoverFilesFindsE2EFile(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"field/fanout/test/e2e.yaml": "spec: {}",
	})

	got, err := DiscoverFiles(dir, "e2e.yaml", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 {
		t.Fatalf("got %v, want one e2e.yaml file", got)
	}
}

func TestIsDiscoveryArg(t *testing.T) {
	cases := []struct {
		arg      string
		wantRoot string
		wantOK   bool
	}{
		{"./...", ".", true},
		{"...", ".", true},
		{".", ".", true},
		{"declarative/...", "declarative", true},
		{"./declarative/...", "./declarative", true},
		{"/abs/path/...", "/abs/path", true},
		{"declarative", "", false},
		{"e2e.yaml", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			root, ok := IsDiscoveryArg(tc.arg)
			if ok != tc.wantOK || root != tc.wantRoot {
				t.Fatalf("IsDiscoveryArg(%q) = (%q, %v), want (%q, %v)",
					tc.arg, root, ok, tc.wantRoot, tc.wantOK)
			}
		})
	}
}

func TestDiscoverFilesFindsE2EFileWithExclusion(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a/e2e.yaml": "spec: a",
		"b/e2e.yaml": "spec: b",
		"c/e2e.yaml": "spec: c",
	})

	got, err := DiscoverFiles(dir, "e2e.yaml", nil, func(path string) bool {
		return filepath.Base(filepath.Dir(path)) == "b"
	})
	if err != nil {
		t.Fatalf("DiscoverFiles: %v", err)
	}

	want := []string{
		filepath.Join(dir, "a", "e2e.yaml"),
		filepath.Join(dir, "c", "e2e.yaml"),
	}

	if len(got) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestToAbs(t *testing.T) {
	wd := t.TempDir()

	tests := []struct {
		name    string
		path    string
		baseDir string
		want    string
	}{
		{
			name:    "absolute path",
			path:    filepath.Join(wd, "file.yaml"),
			baseDir: filepath.Join(wd, "other"),
			want:    filepath.Join(wd, "file.yaml"),
		},
		{
			name:    "relative path with base directory",
			path:    "file.yaml",
			baseDir: wd,
			want:    filepath.Join(wd, "file.yaml"),
		},
		{
			name:    "relative path with nested directory",
			path:    filepath.Join("manifests", "file.yaml"),
			baseDir: wd,
			want:    filepath.Join(wd, "manifests", "file.yaml"),
		},
		{
			name: "relative path without base directory",
			path: "file.yaml",
			want: "", // Checked separately against the current working directory.
		},
		{
			name:    "empty path",
			path:    "",
			baseDir: wd,
			want:    wd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToAbs(tt.path, tt.baseDir)
			if err != nil {
				t.Fatalf("ToAbs() error = %v", err)
			}

			if tt.want == "" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("Getwd() error = %v", err)
				}
				tt.want = filepath.Join(cwd, tt.path)
			}

			if got != tt.want {
				t.Errorf("ToAbs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTempFile(t *testing.T) {
	path, cleanup, err := TempFile("report.json")
	if err != nil {
		t.Fatalf("tempFile: %v", err)
	}
	if filepath.Base(path) != "report.json" {
		t.Fatalf("base = %q, want report.json", filepath.Base(path))
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("temp dir missing: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove dir %q (err=%v)", dir, err)
	}
}

func TestTempFileUniqueDirs(t *testing.T) {
	_, cleanupA, err := TempFile("a.json")
	if err != nil {
		t.Fatalf("tempFile A: %v", err)
	}
	defer cleanupA()

	_, cleanupB, err := TempFile("b.json")
	if err != nil {
		t.Fatalf("tempFile B: %v", err)
	}
	defer cleanupB()

	pathA, _, _ := TempFile("x.json")
	pathB, cleanupC, _ := TempFile("x.json")
	defer cleanupC()

	if filepath.Dir(pathA) == filepath.Dir(pathB) {
		t.Fatalf("two tempFiles shared a dir: %q", filepath.Dir(pathA))
	}
}
