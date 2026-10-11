//go:build !runtime && !gateway

package scaffold

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/examples"
	"github.com/intentd/inrun/pkg/version"
)

//
// ──────────────────────────────────────────────────────────────────────────────
//  Embedded Pack Extraction (default path — no network required)
// ──────────────────────────────────────────────────────────────────────────────

func extractEmbeddedPack(root, pack string) error {
	p, ok := GetPack(pack)
	if !ok {
		return fmt.Errorf("unknown pack %q — run `inrun init --list` to see available packs", pack)
	}
	srcPath := p.Path

	targetDir := filepath.Join(root, filepath.Base(p.Path))

	if err := fs.WalkDir(examples.FS, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		dst := filepath.Join(targetDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		// go.mod.txt and go.sum.txt are renamed at embed time because
		// //go:embed skips subdirectories containing a go.mod (nested modules).
		// Restore the real names transparently on extraction.
		isGoMod := false
		if name := d.Name(); name == "go.mod.txt" || name == "go.sum.txt" {
			dst = filepath.Join(filepath.Dir(dst), strings.TrimSuffix(name, ".txt"))
			isGoMod = name == "go.mod.txt"
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		data, err := examples.FS.ReadFile(path)
		if err != nil {
			return err
		}

		// Strip //go:build ignore added at embed time to keep go vet clean.
		// The tag is only meaningful inside the parent module; the user's
		// extracted project has its own go.mod and must compile normally.
		if filepath.Ext(dst) == ".go" {
			data = bytes.TrimPrefix(data, []byte("//go:build ignore\n\n"))
		}

		// Substitute the Inrun version placeholder so the extracted go.mod
		// always requires the exact version of inrun the user is running.
		if isGoMod {
			data = bytes.ReplaceAll(data, []byte("{{ .InrunVersion }}"), []byte(version.Short()))
		}

		// The embedded FS keeps no file modes; scripts must stay runnable.
		mode := os.FileMode(0644)
		if filepath.Ext(dst) == ".sh" {
			mode = 0755
		}
		return os.WriteFile(dst, data, mode)
	}); err != nil {
		return err
	}

	return nil
}

// extractCanonical writes the canonical hello-website operator into the
// project root in the standard layout: catalog.yaml, manifests/ (CRD and CR)
// and test/ (simulate and e2e specs).
func extractCanonical(root string) error {
	srcPath := "reconcilers/declarative/01-hello-website"
	keep := map[string]bool{
		cmdutil.FileCatalog: true,
		filepath.Join(cmdutil.DirManifests, cmdutil.FileCrd): true,
		filepath.Join(cmdutil.DirManifests, cmdutil.FileCr):  true,
		filepath.Join(cmdutil.DirTest, cmdutil.FileSimulate): true,
		filepath.Join(cmdutil.DirTest, cmdutil.FileE2e):      true,
	}

	return fs.WalkDir(examples.FS, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcPath, path)
		if err != nil || !keep[rel] {
			return err
		}
		data, err := examples.FS.ReadFile(path)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	})
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Download + Cache Example Packs
// ──────────────────────────────────────────────────────────────────────────────
//
// Packs are downloaded from GitHub Releases as:
//
//   examples_<pack>_<version>.tar.gz
//
// They are cached locally under:
//
//   ~/.inrun/packs/
//
// so repeated `inrun init` calls are instant and work offline.
//

func downloadExamplePack(root, pack, version string, refresh bool) error {
	// Resolve cache directory (~/.inrun/packs)
	cache, err := cacheDir()
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("examples_%s_%s.tar.gz", pack, version)
	cachedPath := filepath.Join(cache, filename)
	projectPath := filepath.Join(root, filename)

	// If refresh-cache is set → delete cached file
	if refresh {
		os.Remove(cachedPath)
	}

	// If cached and not refreshing → reuse
	if _, err := os.Stat(cachedPath); err == nil {
		fmt.Printf("    → Using cached pack: %s\n", cachedPath)
		return cmdutil.CopyFile(cachedPath, projectPath)
	}

	// Otherwise download
	url := fmt.Sprintf(
		"https://github.com/intentd/inrun/releases/download/%s/%s",
		version, filename,
	)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Save to cache
	out, err := os.Create(cachedPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	out.Close()

	// Copy to project folder
	return cmdutil.CopyFile(cachedPath, projectPath)
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Extract Example Pack
// ──────────────────────────────────────────────────────────────────────────────
//
// Extracts the tar.gz pack into:
//
//   <project>/examples/<pack>/
//

func extractExamplePack(root, pack, version string) error {
	tarball := filepath.Join(root, fmt.Sprintf("examples_%s_%s.tar.gz", pack, version))

	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()

	// Decompress gzip
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Read tar entries
	tr := tar.NewReader(gzr)

	targetDir := filepath.Join(root, pack)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return err
		}

		path := filepath.Join(targetDir, header.Name)

		switch header.Typeflag {

		// Create directories
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}

		// Create files
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	// Remove tarball from project folder (keep cache intact)
	_ = os.Remove(tarball)

	return nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Helpers
// ──────────────────────────────────────────────────────────────────────────────
//

// cacheDir returns ~/.inrun/packs and ensures it exists.
func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".inrun", "packs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func clearCache() error {
	cache, err := cacheDir()
	if err != nil {
		return err
	}

	fmt.Printf("Clearing cache: %s\n", cache)

	entries, err := os.ReadDir(cache)
	if err != nil {
		return err
	}

	for _, e := range entries {
		os.Remove(filepath.Join(cache, e.Name()))
	}

	fmt.Println("Cache cleared.")
	return nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Init Step Runner (pretty output)
// ──────────────────────────────────────────────────────────────────────────────
//

type initStep struct {
	name string
	fn   func() error
}

// runSteps prints each step and executes it with ✓/✗ feedback.
func runSteps(steps []initStep) error {
	for _, step := range steps {
		fmt.Printf("  %-50s", step.name+"...")
		if err := step.fn(); err != nil {
			fmt.Printf("%s\n", cmdutil.FailureMark())
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Printf("%s\n", cmdutil.SuccessMark())
	}
	return nil
}

// isCurrentDirectory reports whether the project name is "."
func isCurrentDirectory(name string) bool {
	return name == "."
}
