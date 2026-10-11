// pkg/registry/resolve.go
//
// Reference resolution for inrun push, inrun pull, inrun inspect, and inrun patterns.
//
// A bare reference like "postgres:v14" is resolved to a full OCI reference
// using the following priority:
//
//  1. Full OCI reference (starts with "oci://") — used as-is
//  2. INRUN_REGISTRY env var + "/name:version"
//  3. Default: ghcr.io/intentd/registry/patterns/catalogs/name:version
//
// The "oci://" prefix is stripped before passing to ORAS — it is a user-facing
// convention to signal "this is an OCI reference", not part of the actual URL.
//
// CachedDir provides the local cache lookup used by pkg/merger pull helpers.
// Cache layout: ~/.inrun/registry/<host>/<repo>/<version>/
// A hit is declared when catalog.yaml or module.yaml exists in that directory.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ref holds a resolved OCI reference.
type Ref struct {
	// Registry is the hostname (e.g. "ghcr.io").
	Registry string
	// Repository is the full repository path without the registry
	// (e.g. "intentd/inrun-registry/postgres").
	Repository string
	// Tag is the version tag (e.g. "v14").
	Tag string
	// Full is the complete reference without the oci:// prefix.
	Full string
}

// Resolve converts a user-supplied reference to a fully qualified OCI Ref.
//
//	"postgres:v14"                                           → ghcr.io/intentd/registry/patterns/catalogs/postgres:v14
//	"oci://ghcr.io/myorg/patterns/catalogs/redis:v7"        → ghcr.io/myorg/patterns/catalogs/redis:v7
//	"myorg/redis:v7" (with INRUN_REGISTRY set)           → resolved against env
func Resolve(input string) (*Ref, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty reference")
	}

	// Strip oci:// prefix — user-facing convention only
	raw := CleanOCIRef(input)

	// Full reference already provided (contains a dot in the host segment)
	if LooksLikeFullRef(raw) {
		return parseRef(raw)
	}

	// Use INRUN_REGISTRY env var or default
	base := os.Getenv(EnvRegistry)
	if base == "" {
		base = DefaultPatternRegistry
	}
	base = CleanOCIRef(base)

	return parseRef(base + "/" + raw)
}

// looksLikeDigest returns true when s is a valid OCI digest (e.g. "sha256:abc123...").
func looksLikeDigest(s string) bool {
	return strings.HasPrefix(s, "sha256:") || strings.HasPrefix(s, "sha512:") || strings.HasPrefix(s, "sha384:")
}

// LooksLikeFullRef returns true when the reference appears to already contain
// a registry hostname (has a dot or colon before the first slash).
func LooksLikeFullRef(ref string) bool {
	slashIdx := strings.Index(ref, "/")
	if slashIdx < 0 {
		return false
	}
	host := ref[:slashIdx]
	return strings.Contains(host, ".") || strings.Contains(host, ":") || host == "localhost"
}

// parseRef splits a full reference into its components.
func parseRef(full string) (*Ref, error) {
	// Catch @ used as a tag separator — reject only when it's not a valid digest.
	// Valid digest: @sha256:<hex>, @sha512:<hex>, etc.
	// Invalid: @v1.0.0, @latest, @stable (these should use ':')
	if atIdx := strings.LastIndex(full, "@"); atIdx > strings.LastIndex(full, "/") {
		after := full[atIdx+1:]
		if !looksLikeDigest(after) {
			return nil, fmt.Errorf("use ':' not '@' for version tags (e.g. ...:%s) — '@' is for digest references like @sha256:...", after)
		}
		// Valid digest reference — pass through as-is (ORAS handles @sha256: natively)
		slashIdx := strings.Index(full, "/")
		if slashIdx < 0 {
			return nil, fmt.Errorf("invalid reference %q: missing repository path", full)
		}
		return &Ref{
			Registry:   full[:slashIdx],
			Repository: full[slashIdx+1 : atIdx],
			Tag:        after, // store digest in Tag field for cache path use
			Full:       full,
		}, nil
	}

	// Separate tag
	tag := "latest"
	name := full
	if idx := strings.LastIndex(full, ":"); idx > strings.LastIndex(full, "/") {
		tag = full[idx+1:]
		name = full[:idx]
	}

	// Separate registry from repository
	slashIdx := strings.Index(name, "/")
	if slashIdx < 0 {
		return nil, fmt.Errorf("invalid reference %q: missing repository path", full)
	}
	registry := name[:slashIdx]
	repo := name[slashIdx+1:]

	return &Ref{
		Registry:   registry,
		Repository: repo,
		Tag:        tag,
		Full:       full,
	}, nil
}

// CachePath returns the local filesystem path for this ref.
// Structure: ~/.inrun/registry/<registry>/<repository>/<tag>/
func (r *Ref) CachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	// Replace colons in port numbers for filesystem safety
	registry := strings.ReplaceAll(r.Registry, ":", "_")
	repo := filepath.FromSlash(r.Repository)
	return filepath.Join(home, CacheDir, registry, repo, r.Tag), nil
}

// IsCached returns true when the pattern is already in the local cache.
// Checks for either catalog.yaml (pattern) or module.yaml (module).
func (r *Ref) IsCached() bool {
	path, err := r.CachePath()
	if err != nil {
		return false
	}
	for _, file := range []string{FileCatalog, FileModule} {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			return true
		}
	}
	return false
}

// CachedDir returns the local cache directory for an OCI artifact if it has
// been pulled previously. A hit requires catalog.yaml or module.yaml to be
// present — whichever file is found first counts as a complete pull.
//
// ociURL must be the bare host+path without the oci:// prefix or tag,
// e.g. "ghcr.io/intentd/registry/patterns/modules/postgres".
// Returns ("", false) when not cached or on any error.
func CachedDir(ociURL, version string) (string, bool) {
	// Parse into registry + repository components.
	slashIdx := strings.Index(ociURL, "/")
	if slashIdx < 0 {
		return "", false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	reg := strings.ReplaceAll(ociURL[:slashIdx], ":", "_")
	repo := filepath.FromSlash(ociURL[slashIdx+1:])
	dir := filepath.Join(home, CacheDir, reg, repo, version)

	for _, file := range []string{FileCatalog, FileModule} {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return dir, true
		}
	}
	return "", false
}

// String returns the full reference with oci:// prefix for display.
func (r *Ref) String() string {
	return "oci://" + r.Full
}

// ShortName returns the name:tag portion for display.
func (r *Ref) ShortName() string {
	parts := strings.Split(r.Repository, "/")
	return parts[len(parts)-1] + ":" + r.Tag
}

// IsOCIRef reports whether s looks like an OCI pattern reference rather than
// a local filesystem path. Returns true for bare name:tag, full registry
// references, and oci:// URIs. Returns false for absolute paths, relative
// paths (./  ../), and plain filenames without a version tag.
func IsOCIRef(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "oci://") {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return false
	}
	// name:tag or registry/path:tag — colon at position > 1 (avoids Windows C:\)
	if idx := strings.Index(s, ":"); idx > 1 {
		return true
	}
	return LooksLikeFullRef(s)
}

// ResolveForKind resolves a reference against the correct default registry
// for the given pattern kind. A full OCI reference (contains a dot in the host)
// or an oci:// prefix is used as-is regardless of kind.
func ResolveForKind(input string, k PatternKind) (*Ref, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty reference")
	}
	raw := CleanOCIRef(input)
	if LooksLikeFullRef(raw) {
		return parseRef(raw)
	}

	var base string
	switch k {
	case ModuleKind:
		base = os.Getenv(EnvModuleRegistry)
		if base == "" {
			base = DefaultModuleRegistry
		}
	default:
		base = os.Getenv(EnvPatternRegistry)
		if base == "" {
			base = DefaultPatternRegistry
		}
	}
	base = CleanOCIRef(base)
	return parseRef(base + "/" + raw)
}
