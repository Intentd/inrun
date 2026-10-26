package registry

const (
	BlueprintKind PatternKind = "Blueprint" // Blueprint-based operator pattern
	ModuleKind    PatternKind = "Module"    // Reusable resource primitive
	UnknownKind   PatternKind = ""

	// MediaType is the OCI pattern media type for Inrun patterns.
	MediaType = "application/vnd.inrun.pattern.v1+tar+gzip"

	// indexMediaType is the OCI media type for the shared registry index.
	indexMediaType = "application/vnd.inrun.index.v1+json"

	// FileBlueprint is the required operator declaration file.
	FileBlueprint = "blueprint.yaml"

	// FileStack is the optional multi-operator composition file.
	FileStack = "stack.yaml"

	// FileModule is the required module declaration file.
	FileModule = "module.yaml"

	// FileCRD is the required CRD schema file.
	FileCRD = "crd.yaml"

	// FileReadme is the human documentation file.
	FileReadme = "README.md"

	// FileCR is the example CR file.
	FileCR = "cr.yaml"

	// FileE2E is the E2E test definition file for a Blueprint pattern.
	FileE2E = "e2e.yaml"

	// FileSimulate is the simulate spec file for a Blueprint pattern.
	FileSimulate = "simulate.yaml"

	// FileIntentYAML and FileIntentJSON are the serve play intent files for a Blueprint pattern.
	FileIntentYAML = "intent.yaml"
	FileIntentJSON = "intent.json"

	// FileReportMarkdown and FileReportJSON are the e2e report files for a Blueprint pattern.
	FileReportMarkdown = "report.md"
	FileReportJSON     = "report.json"

	// DirManifests and DirTest group a pattern's files: crd.yaml and cr.yaml
	// in manifests/, simulate.yaml and e2e.yaml in test/. The root is also
	// accepted for each.
	DirManifests = "manifests"
	DirTest      = "test"

	// FileGoMod, FileGoSum, and FileMakefile are the typed operator build files.
	// Present only in typed (hooks/constructor) patterns.
	FileGoMod    = "go.mod"
	FileGoSum    = "go.sum"
	FileMakefile = "Makefile"

	// DefaultBlueprintRegistry is the official OCI path for Blueprint patterns.
	DefaultBlueprintRegistry = "ghcr.io/intentd/registry/patterns/blueprints"

	// DefaultModuleRegistry is the official OCI path for Module patterns.
	DefaultModuleRegistry = "ghcr.io/intentd/registry/patterns/modules"

	// DefaultPatternRegistry is an alias for DefaultBlueprintRegistry.
	DefaultPatternRegistry = DefaultBlueprintRegistry

	// EnvPatternRegistry overrides the default blueprint registry path.
	EnvPatternRegistry = "INRUN_REGISTRY"

	// EnvModuleRegistry overrides the default module registry path.
	EnvModuleRegistry = "INRUN_MODULES_REGISTRY"

	// EnvRegistry is an alias for EnvPatternRegistry.
	EnvRegistry = EnvPatternRegistry

	// CacheDir is the local cache directory for pulled artifacts.
	// Resolved relative to the user's home directory.
	CacheDir = ".inrun/registry"

	// HelmGitCacheDir is the local cache directory for git-sourced Helm charts.
	HelmGitCacheDir = ".inrun/helm/git"

	// HelmRepoCacheDir is the local cache directory for remote Helm repository charts.
	HelmRepoCacheDir = ".inrun/helm/repo"

	// FileCacheDir is the local cache directory for remote file fetches (https://).
	FileCacheDir = ".inrun/files"
)
