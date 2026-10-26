// pkg/merger/registry.go
//
// Registry Blueprint imports (imports.registry:) — authoring-time only, same
// reasoning as helm.go: the runtime and gateway only ever read the
// blueprint.yaml key from a ConfigMap, already fully merged by
// `inrun generate bundle` with no imports left to resolve. See registry_stub.go
// for what those two builds get instead.

//go:build !runtime && !gateway

package merger

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/util/fileio"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	pkgregistry "github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
)

const orasPullTimeout = 2 * time.Minute

// knownPatternFiles is every path a pattern file may have, at the root or in
// its group directory. All are attempted during Git pulls; presence is
// validated after pull by validatePatternStructure using the kind-specific
// required/optional rules.
var knownPatternFiles = func() []string {
	var out []string
	for _, f := range []string{
		pkgregistry.FileBlueprint,
		pkgregistry.FileModule,
		pkgregistry.FileCRD,
		pkgregistry.FileReadme,
		pkgregistry.FileCR,
		pkgregistry.FileE2E,
		pkgregistry.FileSimulate,
		pkgregistry.FileGoMod,
		pkgregistry.FileGoSum,
		pkgregistry.FileMakefile,
	} {
		out = append(out, pkgregistry.PatternFilePaths(f)...)
	}
	return out
}()

// loadRegistrySource loads a single registry pattern entry.
//
// Resolution sequence:
//  1. Parse url@version shorthand or url + version fields
//  2. Determine pull method: OCI or Git
//  3. Pull pattern to a temp directory
//  4. Validate pattern structure based on detected kind
//  5. Load blueprint.yaml or stack.yaml based on UseStack
//  6. Parse and return the CRD entries
func (m *Merger) loadRegistrySource(src types.RegistrySource) (map[string]types.CRDEntry, error) {
	cleanURL, version := src.ResolvedURL()

	logger.Debug().
		Str("url", cleanURL).
		Str("version", version).
		Bool("oci", src.IsOCI()).
		Str("loads", src.SourceFile()).
		Msg("merger: pulling registry source")

	auth, err := resolveRegistryAuth(src.Auth)
	if err != nil {
		return nil, fmt.Errorf("registry %q: auth: %w", cleanURL, err)
	}

	tmpDir, cleanup, err := m.pullPattern(cleanURL, version, src.IsOCI(), auth)
	if err != nil {
		return nil, fmt.Errorf("registry %q@%s: pull failed: %w", cleanURL, version, err)
	}
	defer cleanup()

	if err := validatePatternStructure(tmpDir, cleanURL, version); err != nil {
		return nil, err
	}

	sourceFile := filepath.Join(tmpDir, src.SourceFile())
	sourcePath := fmt.Sprintf("registry:%s@%s/%s", cleanURL, version, src.SourceFile())

	data, err := readLocal(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("registry %q@%s: reading %s: %w",
			cleanURL, version, src.SourceFile(), err)
	}

	doc, err := parseBlueprintDoc(data, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("registry %q@%s: parsing %s: %w",
			cleanURL, version, src.SourceFile(), err)
	}
	if doc == nil {
		return nil, fmt.Errorf(
			"registry %q@%s: %s is not a valid Blueprint or Stack document",
			cleanURL, version, src.SourceFile(),
		)
	}

	if lc := doc.Lifecycle; lc != nil {
		if dep := lc.Deprecation; dep != nil {
			msg := fmt.Sprintf("warning: registry pattern %q@%s is deprecated", cleanURL, version)
			if dep.MigratedTo != "" {
				msg += fmt.Sprintf(" — migrate to: %s", dep.MigratedTo)
			}
			if dep.Message != "" {
				msg += fmt.Sprintf(" (%s)", dep.Message)
			}
			fmt.Fprintln(os.Stderr, msg)
		}
	}

	registryRef := fmt.Sprintf("%s@%s", cleanURL, version)

	switch doc.Kind {
	case config.BlueprintKind():
		if src.UseStack {
			return nil, fmt.Errorf(
				"registry %q@%s: useStack is true but stack.yaml contains kind %q — "+
					"check the upstream pattern's stack.yaml",
				cleanURL, version, doc.Kind,
			)
		}
		// The real path, not the display label: relative crdFile, crFiles and
		// module paths resolve against the pattern's directory.
		entries, err := m.loadBlueprint(sourceFile, doc)
		if err != nil {
			return nil, err
		}
		stampRegistryRef(entries, registryRef)
		return entries, nil

	case config.StackKind():
		if !src.UseStack {
			return nil, fmt.Errorf(
				"registry %q@%s: useStack is false but blueprint.yaml contains kind %q — "+
					"set useStack: true to load the upstream Stack, or check the pattern structure",
				cleanURL, version, doc.Kind,
			)
		}
		entries, err := m.loadStack(sourceFile, doc)
		if err != nil {
			return nil, err
		}
		stampRegistryRef(entries, registryRef)
		return entries, nil

	default:
		return nil, fmt.Errorf(
			"registry %q@%s: %s has unexpected kind %q — expected %q or %q",
			cleanURL, version, src.SourceFile(), doc.Kind,
			config.BlueprintKind(), config.StackKind(),
		)
	}
}

// stampRegistryRef sets RegistryRef on every entry in the map.
func stampRegistryRef(entries map[string]types.CRDEntry, ref string) {
	for name, entry := range entries {
		entry.RegistryRef = ref
		entries[name] = entry
	}
}

// pullPattern returns the directory of a registry pattern and a cleanup
// function: the local cache for OCI (cleanup is a no-op), a temp dir for Git.
func (m *Merger) pullPattern(
	url, version string,
	oci bool,
	auth *fileio.FileAuth,
) (tmpDir string, cleanup func(), err error) {
	// OCI patterns load from the local cache, which outlives this call: the
	// pattern's relative files (manifests/crd.yaml, ...) are read after loading.
	if oci {
		dir, err := fetchOCI(url, version, auth)
		if err != nil {
			return "", nil, err
		}
		return dir, noopCleanup, nil
	}

	tmpDir, err = os.MkdirTemp("", "inrun-registry-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	cleanup = func() { os.RemoveAll(tmpDir) }

	if err = m.pullGitPattern(url, version, tmpDir, auth); err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpDir, cleanup, nil
}

// ── OCI pull ──────────────────────────────────────────────────────────────────

// fetchOCI returns the local cache directory of an OCI pattern or module,
// pulling it first when needed. registry.Fetch is the one place artifacts are
// fetched; this only adapts an import's auth block.
func fetchOCI(url, version string, auth *fileio.FileAuth) (string, error) {
	ociRef := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	ociRef = strings.TrimSuffix(ociRef, "/")
	ref, err := pkgregistry.Resolve(fmt.Sprintf("%s:%s", ociRef, version))
	if err != nil {
		return "", fmt.Errorf("OCI reference %q: %w", ociRef, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), orasPullTimeout)
	defer cancel()
	return pkgregistry.Fetch(ctx, ref, false, registryCredentials(auth))
}

// registryCredentials converts an import's auth block. Nil means the Docker
// credential store; a block without usable credentials pulls anonymously.
func registryCredentials(a *fileio.FileAuth) *pkgregistry.Credentials {
	if a == nil {
		return nil
	}
	switch strings.ToLower(a.Type) {
	case "basic":
		if a.Username != "" && a.Password != "" {
			return &pkgregistry.Credentials{Username: a.Username, Password: a.Password}
		}
	case "bearer", "github":
		if a.BearerToken != "" {
			return &pkgregistry.Credentials{Token: a.BearerToken}
		}
	}
	return &pkgregistry.Credentials{}
}

// ── Git pull ──────────────────────────────────────────────────────────────────

func (m *Merger) pullGitPattern(url, version, tmpDir string, auth *fileio.FileAuth) error {
	switch {
	case isGitHubURL(url):
		return m.pullGitHubPattern(url, version, tmpDir, auth)
	case isGitLabURL(url):
		return m.pullGitLabPattern(url, version, tmpDir, auth)
	default:
		return pullGenericGitPattern(url, version, tmpDir, auth)
	}
}

// pullGitHubPattern fetches all known pattern files from a GitHub repository.
// Files that don't exist are silently skipped; validatePatternStructure enforces
// the kind-specific required set after the pull.
func (m *Merger) pullGitHubPattern(url, version, tmpDir string, auth *fileio.FileAuth) error {
	for _, filename := range knownPatternFiles {
		rawURL := githubRawURL(url, version, filename)
		data, err := fileio.LoadFileWithAuth(rawURL, auth)
		if err != nil {
			continue // file not present in this pattern — validated after pull
		}
		if err := writePatternFile(tmpDir, filename, data); err != nil {
			return fmt.Errorf("writing %q: %w", filename, err)
		}
	}
	return nil
}

// pullGitLabPattern fetches all known pattern files from a GitLab repository.
func (m *Merger) pullGitLabPattern(url, version, tmpDir string, auth *fileio.FileAuth) error {
	for _, filename := range knownPatternFiles {
		rawURL := gitlabRawURL(url, version, filename)
		data, err := fileio.LoadFileWithAuth(rawURL, auth)
		if err != nil {
			continue
		}
		if err := writePatternFile(tmpDir, filename, data); err != nil {
			return fmt.Errorf("writing %q: %w", filename, err)
		}
	}
	return nil
}

// pullGenericGitPattern clones the repository and copies all known pattern files.
func pullGenericGitPattern(url, version, tmpDir string, auth *fileio.FileAuth) error {
	cloneDir, err := os.MkdirTemp("", "inrun-clone-*")
	if err != nil {
		return fmt.Errorf("creating clone dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	cloneURL := injectAuthIntoURL(url, auth)
	if err := gitClone(cloneURL, cloneDir, version); err != nil {
		return err
	}

	for _, filename := range knownPatternFiles {
		data, err := readLocal(filepath.Join(cloneDir, filename))
		if err != nil {
			continue
		}
		if err := writePatternFile(tmpDir, filename, data); err != nil {
			return fmt.Errorf("copying %q: %w", filename, err)
		}
	}

	return nil
}

// pullModuleFromGit fetches only module.yaml from a Git host.
// Used for standalone Module repos (not full patterns).
func (m *Merger) pullModuleFromGit(url, version, tmpDir string, auth *fileio.FileAuth) error {
	var rawURL string
	switch {
	case isGitHubURL(url):
		rawURL = githubRawURL(url, version, "module.yaml")
	case isGitLabURL(url):
		rawURL = gitlabRawURL(url, version, "module.yaml")
	default:
		return m.fetchModuleFromGenericGit(url, version, tmpDir, auth)
	}

	data, err := fileio.LoadFileWithAuth(rawURL, auth)
	if err != nil {
		return fmt.Errorf("fetching module.yaml from %s@%s: %w", url, version, err)
	}
	return os.WriteFile(filepath.Join(tmpDir, "module.yaml"), data, 0644)
}

func (m *Merger) fetchModuleFromGenericGit(url, version, tmpDir string, auth *fileio.FileAuth) error {
	cloneDir, err := os.MkdirTemp("", "inrun-module-clone-*")
	if err != nil {
		return fmt.Errorf("creating clone dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	cloneURL := injectAuthIntoURL(url, auth)
	if err := gitClone(cloneURL, cloneDir, version); err != nil {
		return err
	}

	data, err := readLocal(filepath.Join(cloneDir, "module.yaml"))
	if err != nil {
		return fmt.Errorf("module.yaml not found in repository %s@%s", url, version)
	}
	return os.WriteFile(filepath.Join(tmpDir, "module.yaml"), data, 0644)
}

// ── Pattern validation ────────────────────────────────────────────────────────

// validatePatternStructure validates that dir contains a well-formed pattern.
// Kind is auto-detected; required files are determined by the pattern kind.
//
// Blueprint patterns require only blueprint.yaml (crd.yaml, README.md, cr.yaml are optional).
// Module patterns require only module.yaml.
func validatePatternStructure(dir, url, version string) error {
	_, _, _, err := pkgregistry.ValidatePatternDirectory(dir)
	if err != nil {
		return fmt.Errorf("registry pattern %q@%s failed structure validation: %w", url, version, err)
	}
	return nil
}

// ── URL construction ──────────────────────────────────────────────────────────

func isGitHubURL(u string) bool {
	return strings.Contains(u, "github.com") && !strings.HasSuffix(u, ".git")
}

func isGitLabURL(u string) bool {
	return strings.Contains(u, "gitlab.com") && !strings.HasSuffix(u, ".git")
}

func githubRawURL(repoURL, ref, filePath string) string {
	u := strings.TrimSuffix(strings.TrimSuffix(repoURL, ".git"), "/")
	u = strings.Replace(u, "https://github.com/", "https://raw.githubusercontent.com/", 1)
	u = strings.Replace(u, "http://github.com/", "https://raw.githubusercontent.com/", 1)
	return fmt.Sprintf("%s/%s/%s", u, ref, filePath)
}

func gitlabRawURL(repoURL, ref, filePath string) string {
	u := strings.TrimSuffix(strings.TrimSuffix(repoURL, ".git"), "/")
	return fmt.Sprintf("%s/-/raw/%s/%s", u, ref, filePath)
}

func injectAuthIntoURL(rawURL string, auth *fileio.FileAuth) string {
	if auth == nil || strings.ToLower(auth.Type) != "basic" || auth.Username == "" {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsed.User = url.UserPassword(auth.Username, auth.Password)
	return parsed.String()
}

// ── Auth resolution ───────────────────────────────────────────────────────────

func resolveRegistryAuth(auth *types.FileSourceAuth) (*fileio.FileAuth, error) {
	if auth == nil {
		return nil, nil
	}
	return auth.Resolve()
}

// writePatternFile writes a pulled pattern file under dir, creating its group
// directory when the path has one.
func writePatternFile(dir, rel string, data []byte) error {
	dst := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
