// pkg/registry/imports.go
//
// Extracts OCI references from a blueprint or stack file so they can be
// pre-pulled before running validate, generate, or run commands.
package registry

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// OCIImports holds all OCI references found in a blueprint or stack file.
type OCIImports struct {
	ModuleImports   []types.ModuleImport
	RegistrySources []types.RegistrySource
}

// Empty returns true when there are no OCI refs to pull.
func (o *OCIImports) Empty() bool {
	return len(o.ModuleImports) == 0 && len(o.RegistrySources) == 0
}

// ExtractOCIImports parses a blueprint or stack file and returns all OCI
// references that must be pre-pulled before validate/generate/run.
//
// For Blueprint files: spec.crds[*].imports[*].module refs that resolve to OCI.
// For Stack files: imports.registry[*] sources where oci: true or the URL
// has an oci:// prefix.
//
// Module shorthands (bare names like "postgres" or "postgres:v0.1.0") are
// treated as OCI — they resolve against the default module registry, same
// as the LoadImport resolution order.
func ExtractOCIImports(filePath string) (*OCIImports, error) {
	data, err := readLocal(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", filePath, err)
	}

	var kf types.BlueprintFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filePath, err)
	}

	out := &OCIImports{}

	// Module imports from spec.crds[*].imports (Blueprint and inline Stack CRDs)
	for _, crd := range kf.Spec.CRDs {
		for _, imp := range crd.EffectiveImports() {
			if isOCIModuleImport(imp) {
				out.ModuleImports = append(out.ModuleImports, imp)
			}
		}
	}

	// Registry imports from imports.registry (Stack)
	if kf.Imports != nil {
		for _, src := range kf.Imports.Registry {
			if isOCIRegistrySource(src) {
				out.RegistrySources = append(out.RegistrySources, src)
			}
		}
	}

	return out, nil
}

// HelmAndFileImports holds non-OCI remote sources that benefit from local caching:
// git/remote Helm sources and HTTPS file sources from a stack spec.
type HelmAndFileImports struct {
	// HelmSources are all helm: entries in imports.helm.
	HelmSources []types.HelmSource
	// RemoteFiles are all HTTPS URLs found in imports.files.
	RemoteFiles []string
}

// Empty returns true when there are no cacheable remote sources.
func (h *HelmAndFileImports) Empty() bool {
	return len(h.HelmSources) == 0 && len(h.RemoteFiles) == 0
}

// ExtractHelmAndFileImports parses a stack file and returns all helm sources
// and HTTPS file sources. Local file paths are excluded — they need no caching.
func ExtractHelmAndFileImports(filePath string) (*HelmAndFileImports, error) {
	data, err := readLocal(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", filePath, err)
	}

	var kf types.BlueprintFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filePath, err)
	}

	out := &HelmAndFileImports{}

	if kf.Imports == nil {
		return out, nil
	}

	out.HelmSources = append(out.HelmSources, kf.Imports.Helm...)

	for _, fs := range kf.Imports.Files {
		if strings.HasPrefix(fs.URL, "https://") || strings.HasPrefix(fs.URL, "http://") {
			out.RemoteFiles = append(out.RemoteFiles, fs.URL)
		}
	}

	return out, nil
}

// LocalModuleImport describes a module import in a blueprint that uses a local file path.
// Local imports are valid for development (inrun simulate, inrun template) but cannot
// be resolved by consumers after the blueprint is published.
type LocalModuleImport struct {
	CRDName string
	Index   int
	Path    string
}

// ExtractLocalModuleImports parses a blueprint.yaml and returns any module imports
// that reference local file paths. The caller should block inrun push when the
// result is non-empty and prompt the user to replace them with OCI refs.
func ExtractLocalModuleImports(filePath string) ([]LocalModuleImport, error) {
	data, err := readLocal(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", filePath, err)
	}
	var kf types.BlueprintFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filePath, err)
	}
	var out []LocalModuleImport
	for crdName, crd := range kf.Spec.CRDs {
		for i, imp := range crd.EffectiveImports() {
			if ref := strings.TrimSpace(imp.Module); IsFilePath(ref) {
				out = append(out, LocalModuleImport{CRDName: crdName, Index: i, Path: ref})
			}
		}
	}
	return out, nil
}

// isOCIModuleImport mirrors the resolution order in pkg/module.LoadImport:
//  1. File path → not OCI
//  2. Git URL → not OCI
//  3. oci:// prefix → OCI
//  4. oci: true → OCI
//  5. Bare name (no dots in host, not a git URL) → OCI (resolved against default module registry)
//  6. Full ref without oci: true → not OCI (requires explicit oci flag)
func isOCIModuleImport(imp types.ModuleImport) bool {
	ref := strings.TrimSpace(imp.Module)
	if ref == "" {
		return false
	}
	if IsFilePath(ref) || isModuleGitURL(ref) {
		return false
	}
	if strings.HasPrefix(ref, "oci://") || imp.OCI {
		return true
	}
	// Bare name: no dots in the host segment before the first slash, not a full ref.
	// e.g. "postgres", "postgres:v0.1.0" — resolves against default module registry.
	return !LooksLikeFullRef(ref)
}

// isOCIRegistrySource returns true when a RegistrySource resolves via OCI.
// Explicit oci: true flag or an oci:// prefix on the URL both count.
func isOCIRegistrySource(src types.RegistrySource) bool {
	return src.OCI || strings.HasPrefix(strings.TrimSpace(src.URL), "oci://")
}

// isModuleGitURL mirrors isGitURL in pkg/module/loader.go.
func isModuleGitURL(ref string) bool {
	return strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "git@")
}
