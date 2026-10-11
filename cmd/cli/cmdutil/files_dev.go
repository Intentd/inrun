//go:build !runtime && !gateway

package cmdutil

// Add any function that requires dev-only tools or symbols (startSpinner, successMark,
// pkg/registry, etc.) here. This file is excluded from runtime and gateway builds.

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
)

// ResolveOCIRunPath resolves an OCI pattern reference to a local catalog or
// stack path, pulling to the cache if necessary.
func ResolveOCIRunPath(ctx context.Context, ref string, useStack, refresh bool) (string, error) {
	r, err := registry.ResolveForKind(ref, registry.CatalogKind)
	if err != nil {
		return "", fmt.Errorf("invalid reference: %w", err)
	}

	if !r.IsCached() || refresh {
		client, err := registry.NewClient()
		if err != nil {
			return "", fmt.Errorf("initializing registry client: %w", err)
		}
		fmt.Printf("Pulling %s\n  → %s\n", r.ShortName(), r.String())
		spin := StartSpinner("Downloading...")
		if _, err := client.Pull(ctx, r, refresh); err != nil {
			spin.Failure()
			return "", fmt.Errorf("pull failed: %w", err)
		}
		spin.Stop()
		fmt.Printf("  %s Cached\n", SuccessMark())
	}

	cacheDir, err := r.CachePath()
	if err != nil {
		return "", err
	}

	target := FileCatalog
	if useStack {
		target = fileStack
	}
	p := filepath.Join(cacheDir, target)
	if !FileExists(p) {
		return "", fmt.Errorf("%s not found in cached pattern %s", target, r.ShortName())
	}
	return p, nil
}
