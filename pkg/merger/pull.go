// pkg/merger/pull.go
//
// Exported pull helpers — shared with pkg/module and any other package
// that needs to fetch a registry artifact without loading a full Blueprint.
//
// Both PullToDir and PullModuleToDir follow a cache-first strategy for OCI
// artifacts: ~/.inrun/registry/<host>/<repo>/<version>/ is checked for a
// sentinel file (blueprint.yaml or module.yaml) before any network call is made.
// This avoids redundant pulls and removes the need for Docker credential
// forwarding inside the process — callers should use `inrun pull`
// to populate the cache, then rely on these helpers to read from it.
//
// Authoring-time only, same as registry.go: depends on its pullPattern /
// pullModuleFromGit methods, which don't exist in the runtime/gateway
// builds. See pull_stub.go.

//go:build !runtime && !gateway

package merger

import (
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/util/fileio"

	"github.com/intentd/inrun/pkg/registry"
)

// noop cleanup used when serving from cache — nothing to remove.
var noopCleanup = func() {}

// PullToDir returns the directory for a registry pattern artifact (OCI or Git).
// For OCI artifacts it checks the local cache (~/.inrun/registry/) first and
// returns the cached directory without a network call when available.
//
// Returns (dir, cleanup, err). Always call cleanup() when done —
// for cached hits cleanup is a no-op; for fresh pulls it removes the temp dir.
func PullToDir(url, version string, oci bool, auth *fileio.FileAuth) (dir string, cleanup func(), err error) {
	if oci {
		if cached, ok := registry.CachedDir(url, version); ok {
			return cached, noopCleanup, nil
		}
	}
	m := &Merger{}
	return m.pullPattern(url, version, oci, auth)
}

// PullModuleToDir returns the directory for a module artifact (OCI or Git).
// For OCI artifacts it checks the local cache (~/.inrun/registry/) first and
// returns the cached directory without a network call when available.
//
// For OCI: the entire OCI artifact is pulled — module.yaml must be at the root.
// For Git (GitHub/GitLab): only module.yaml is fetched via raw URL.
// For generic Git: the repo is cloned and module.yaml is copied.
//
// Returns (dir, cleanup, err). Always call cleanup() when done.
func PullModuleToDir(url, version string, oci bool, auth *fileio.FileAuth) (dir string, cleanup func(), err error) {
	if oci {
		if cached, ok := registry.CachedDir(url, version); ok {
			return cached, noopCleanup, nil
		}
	}

	tmpDir, err := os.MkdirTemp("", "inrun-module-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmpDir) }

	m := &Merger{}
	if oci {
		err = m.pullOCIPattern(url, version, tmpDir, auth)
	} else {
		err = m.pullModuleFromGit(url, version, tmpDir, auth)
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpDir, cleanup, nil
}
