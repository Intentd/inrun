// pkg/module/pull.go
//
// PullImport fetches a module OCI artifact to the local registry cache.
// Used as the pre-pull step before validate, generate, or run commands.
//
// Authoring-time only: every caller (inrun pull, inrun e2e) is already
// !runtime && !gateway tagged. LoadImport in loader.go (not gated) is the
// path pkg/catalog's module-import expansion actually uses, and it goes
// through merger.PullModuleToDir, not this file.

//go:build !runtime && !gateway

package module

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
)

// PullImport fetches a module artifact into the registry cache
// (~/.inrun/registry/...) so subsequent loads are served from disk.
// Non-OCI refs (file paths, git URLs) are silently skipped.
// Resolution mirrors LoadImport exactly, including bare-name → default registry expansion.
func PullImport(imp *types.ModuleImport) error {
	ref := strings.TrimSpace(imp.Module)

	if registry.IsFilePath(ref) || isGitURL(ref) {
		return nil
	}

	oci := imp.OCI
	if registry.IsOCIRef(ref) {
		oci = true
		ref = registry.CleanOCIRef(ref)
	}

	// Bare name → resolve against default module registry.
	var resolved *registry.Ref
	if !oci && !registry.LooksLikeFullRef(ref) {
		r, err := registry.ResolveForKind(ref, registry.ModuleKind)
		if err != nil {
			return fmt.Errorf("module %q: resolving reference: %w", imp.Module, err)
		}
		resolved = r
	} else if oci {
		cleanURL, version := resolveModuleRef(ref, imp.Version, true)
		r, err := registry.Resolve(fmt.Sprintf("%s:%s", cleanURL, version))
		if err != nil {
			return fmt.Errorf("module %q: resolving reference: %w", imp.Module, err)
		}
		resolved = r
	} else {
		return nil // full ref without oci: true — not an OCI import
	}

	if resolved.IsCached() {
		return nil
	}

	client, err := registry.NewClient()
	if err != nil {
		return fmt.Errorf("module %q: initializing client: %w", imp.Module, err)
	}
	if _, err := client.Pull(context.Background(), resolved, false); err != nil {
		return fmt.Errorf("module %q: pull failed: %w", imp.Module, err)
	}
	return nil
}
