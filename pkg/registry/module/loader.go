// pkg/module/loader.go
//
// Loads a Module from a file path or registry reference.
//
// Resolution mirrors RegistrySource in a Stack exactly — the same four
// reference forms are supported:
//
//	module: postgres                                            # bare name → default module registry (OCI)
//	module: oci://ghcr.io/intentd/registry/modules/postgres:v0.1.0   # oci:// prefix → OCI
//	module: ghcr.io/intentd/registry/modules/postgres@v0.1.0          # full OCI ref with oci: true
//	module: https://github.com/myorg/postgres-module@main              # git URL
//	module: ./modules/postgres/module.yaml                              # file path
//
// Bare names and oci:// prefixes are auto-detected so oci: true is not required
// for those forms. For full OCI refs (host with dots), oci: true is still required
// — same as RegistrySource.
package module

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal       = fileio.ReadLocal
	strictUnmarshal = structured.StrictUnmarshal
)

// Load loads a Module from a local file path.
// For full import resolution (registry, OCI, auth), use LoadImport.
func Load(path string) (*types.Module, error) {
	data, err := readLocal(path)
	if err != nil {
		return nil, fmt.Errorf("reading module %s: %w", path, err)
	}
	m, err := parse(data)
	if err != nil {
		return nil, err
	}
	if err := expandIncludes(m, filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("module %s: %w", path, err)
	}
	return m, nil
}

// LoadImport resolves and loads a Module from a ModuleImport declaration.
//
// Resolution order:
//  1. File path (starts with ./, ../, /, or ends with .yaml/.yml)
//  2. oci:// prefix → OCI pull (auto-detected, oci: field not required)
//  3. Bare name (no scheme, no dots in registry host) → resolved against the
//     default module registry (INRUN_MODULES_REGISTRY or ghcr.io/intentd/registry/modules)
//  4. Full OCI ref + oci: true → OCI pull (stack-compatible form)
//  5. Git URL (https://, http://, git@) → git pull
func LoadImport(imp *types.ModuleImport) (*types.Module, error) {
	ref := strings.TrimSpace(imp.Module)

	// File path — relative, absolute, or ends with .yaml/.yml
	if registry.IsFilePath(ref) {
		return Load(ref)
	}

	oci := imp.OCI

	// oci:// prefix → always OCI, strip prefix before further parsing.
	if registry.IsOCIRef(ref) {
		oci = true
		ref = registry.CleanOCIRef(ref)
	}

	// Bare name — no scheme, no dots in the host segment → resolve against
	// the default module registry and pull via OCI.
	// e.g. "postgres" or "postgres:v0.1.0"
	if !oci && !isGitURL(ref) && !registry.LooksLikeFullRef(ref) {
		resolved, err := registry.ResolveForKind(ref, registry.ModuleKind)
		if err != nil {
			return nil, fmt.Errorf("module %q: resolving reference: %w", imp.Module, err)
		}
		ref = resolved.Full // e.g. "ghcr.io/intentd/registry/modules/postgres:v0.1.0"
		oci = true
	}

	// Parse cleanURL and version.
	// Supports both OCI's :tag syntax and the @version shorthand used by RegistrySource.
	cleanURL, version := resolveModuleRef(ref, imp.Version, oci)

	auth, err := imp.Auth.Resolve()
	if err != nil {
		return nil, fmt.Errorf("module %q: auth: %w", imp.Module, err)
	}

	tmpDir, cleanup, err := merger.PullModuleToDir(cleanURL, version, oci, auth)
	if err != nil {
		return nil, fmt.Errorf("module %q@%s: pull failed: %w", cleanURL, version, err)
	}
	defer cleanup()

	fileName := registry.FileModule
	data, err := readLocal(filepath.Join(tmpDir, fileName))
	if err != nil {
		return nil, fmt.Errorf("module %q@%s: %s not found in artifact: %w", cleanURL, version, fileName, err)
	}

	return parse(data)
}

// isGitURL reports whether ref is a Git remote URL.
func isGitURL(ref string) bool {
	return strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "git@")
}

// resolveModuleRef returns the (cleanURL, version) pair ready for PullModuleToDir.
//
// Precedence:
//  1. @version shorthand in ref  — "ghcr.io/.../postgres@v14"
//  2. :tag at the end of an OCI ref — "ghcr.io/.../postgres:v14"
//  3. Explicit imp.Version field
//  4. Default: "latest" for OCI, "main" for Git
func resolveModuleRef(ref, version string, oci bool) (cleanURL, resolvedVersion string) {
	// @ shorthand (stack style) — takes precedence over everything.
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}

	// For OCI refs, a colon after the last slash is the image tag.
	// e.g. "ghcr.io/intentd/registry/modules/postgres:v0.1.0"
	if oci {
		if colonIdx := strings.LastIndex(ref, ":"); colonIdx > strings.LastIndex(ref, "/") {
			return ref[:colonIdx], ref[colonIdx+1:]
		}
	}

	// Explicit version field or defaults.
	cleanURL = ref
	if version != "" {
		return cleanURL, version
	}
	if oci {
		return cleanURL, "latest"
	}
	return cleanURL, "main"
}

func parse(data []byte) (*types.Module, error) {
	var m types.Module
	if err := strictUnmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing module: %w", err)
	}
	if !config.IsModuleKind(m.Kind) {
		return nil, fmt.Errorf("expected kind: Module, got: %s", m.Kind)
	}
	if m.Metadata.Name == "" {
		return nil, fmt.Errorf("module metadata.name is required")
	}
	return &m, nil
}
