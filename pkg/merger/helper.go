// pkg/merger/helper.go
package merger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal        = fileio.ReadLocal
	strictUnmarshal  = structured.StrictUnmarshal
	loadFile         = fileio.LoadFile
	loadFileWithAuth = fileio.LoadFileWithAuth
	resolveEnvVar    = structured.ResolveEnvVar
)

// mergeBlueprintSecurity merges two BlueprintSecurity values.
// Fields that are non-nil/non-zero in override win; otherwise the base value is kept.
// This is the correct semantics for Stack layering: source Blueprint settings are
// inherited, and the Stack only needs to declare what it explicitly wants to change.
func mergeBlueprintSecurity(base, override types.BlueprintSecurity) types.BlueprintSecurity {
	result := base
	if override.DeletionProtection != nil {
		result.DeletionProtection = override.DeletionProtection
	}
	if override.Webhooks != nil {
		result.Webhooks = override.Webhooks
	}
	if override.Conversion != nil {
		result.Conversion = override.Conversion
	}
	if override.NamespaceProtection != nil {
		result.NamespaceProtection = override.NamespaceProtection
	}
	if override.ServiceName != nil {
		result.ServiceName = override.ServiceName
	}
	return result
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (m *Merger) mustBeMerged() {
	if !m.merged {
		panic("merger: call Merge() before querying")
	}
}

// checkDuplicate returns an error if name is already in seen from a different source.
func checkDuplicate(seen map[string]string, name, source string) error {
	if existing, ok := seen[name]; ok && existing != source {
		return fmt.Errorf(
			"duplicate CRD %q: defined in %q and %q — names must be unique across all imports",
			name, existing, source,
		)
	}
	return nil
}

// writeTempFile writes data to a temp file and returns the path.
func writeTempFile(data []byte, pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	return f.Name(), nil
}

// gitClone clones a git repository into dst at the given ref.
// ref may be a branch, tag, or commit hash.
func gitClone(repo, dst, ref string) error {
	if ref == "" {
		ref = "HEAD"
	}

	// First attempt: shallow clone at branch/tag
	cmd := exec.Command("git", "clone",
		"--depth", "1",
		"--branch", ref,
		repo,
		dst,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: full clone + checkout (for commit hashes)
	if err := exec.Command("git", "clone", repo, dst).Run(); err != nil {
		return fmt.Errorf("git clone failed for %q: %w", repo, err)
	}

	if err := exec.Command("git", "-C", dst, "checkout", ref).Run(); err != nil {
		return fmt.Errorf("git checkout %q failed in %q: %w", ref, repo, err)
	}

	return nil
}

// isFileModule returns true when a module reference is a local file path.
// Matches ./, ../, absolute paths, and bare relative paths (e.g. "modules/foo/module.yaml").
// OCI references (ghcr.io/...) and URLs (https://...) do not match.
func isFileModule(module string) bool {
	return strings.HasPrefix(module, "./") ||
		strings.HasPrefix(module, "../") ||
		filepath.IsAbs(module) ||
		(!strings.Contains(module, "://") && !strings.Contains(module, ":") && strings.Contains(module, "/"))
}

// unused but kept for completeness
var _ = bytes.NewBuffer
