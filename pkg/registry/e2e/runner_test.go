package e2e

import (
	"path/filepath"
	"testing"
)

func TestRunnerAbs(t *testing.T) {
	wd := t.TempDir()

	r := &Runner{
		e2eDir: wd,
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "absolute path",
			path: filepath.Join(wd, "manifest.yaml"),
			want: filepath.Join(wd, "manifest.yaml"),
		},
		{
			name: "relative path",
			path: "manifest.yaml",
			want: filepath.Join(wd, "manifest.yaml"),
		},
		{
			name: "nested relative path",
			path: filepath.Join("manifests", "manifest.yaml"),
			want: filepath.Join(wd, "manifests", "manifest.yaml"),
		},
		{
			name: "relative path with parent directory",
			path: filepath.Join("..", "manifest.yaml"),
			want: filepath.Join(filepath.Dir(wd), "manifest.yaml"),
		},
		{
			name: "empty path",
			path: "",
			want: wd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.abs(tt.path)
			if got != tt.want {
				t.Errorf("abs(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
