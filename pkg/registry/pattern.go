// pkg/registry/pattern.go
//
// Generic pattern layer for the Inrun registry.
//
// Every Inrun pattern file carries a kind: field. This package reads that
// field to determine the pattern's media type, required/optional files, and
// which registry to push to — without a separate code path per kind.
//
// To add a new pattern kind: add one entry to patternSpecs. Nothing else changes.
package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// patternSpecs is the registry of known pattern kinds.
// Add new kinds here — no other changes required.
var patternSpecs = map[PatternKind]*PatternSpec{
	BlueprintKind: {
		Kind:          BlueprintKind,
		MediaType:     "application/vnd.inrun.pattern.v1+tar+gzip",
		PrimaryFile:   FileBlueprint,
		RequiredFiles: []string{FileBlueprint},
		OptionalFiles: []string{
			FileStack, FileCRD, FileCR,
			FileE2E, FileSimulate, FileReadme,
			FileIntentYAML, FileIntentJSON,
			FileGoMod, FileGoSum, FileMakefile,
			FileReportMarkdown, FileReportJSON,
		},
	},
	ModuleKind: {
		Kind:          ModuleKind,
		MediaType:     "application/vnd.inrun.module.v1+tar+gzip",
		PrimaryFile:   FileModule,
		RequiredFiles: []string{FileModule},
		OptionalFiles: []string{FileReadme, "example/"},
	},
}

// DetectKind reads the primary YAML file in dir and returns the pattern kind.
// Tries blueprint.yaml first, then module.yaml.
func DetectKind(dir string) (PatternKind, *PatternSpec, error) {
	candidates := []string{FileBlueprint, FileModule}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		data, err := readLocal(path)
		if err != nil {
			continue
		}
		var header struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal(data, &header); err != nil {
			return UnknownKind, nil, fmt.Errorf("reading %s: %w", name, err)
		}
		kind := PatternKind(header.Kind)
		if spec, ok := patternSpecs[kind]; ok {
			return kind, spec, nil
		}
	}
	return UnknownKind, nil, fmt.Errorf(
		"no recognized Inrun pattern in %s (expected %s with kind: Blueprint, or module.yaml with kind: Module)",
		dir, FileBlueprint,
	)
}

// SpecFor returns the PatternSpec for a given kind.
func SpecFor(kind PatternKind) (*PatternSpec, error) {
	spec, ok := patternSpecs[kind]
	if !ok {
		return nil, fmt.Errorf("unknown pattern kind: %q", kind)
	}
	return spec, nil
}

// ValidatePatternDirectory validates that dir contains a well-formed pattern
// of the auto-detected kind. Returns the kind, spec, and the list of files to include.
func ValidatePatternDirectory(dir string, includeReport bool) (PatternKind, *PatternSpec, []string, error) {
	kind, spec, err := DetectKind(dir)
	if err != nil {
		return UnknownKind, nil, nil, err
	}

	var files []string
	for _, f := range spec.RequiredFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			return kind, spec, nil, fmt.Errorf("%s pattern missing required file: %s", kind, f)
		}
		files = append(files, f)
	}
	for _, f := range spec.OptionalFiles {
		if !includeReport && f == FileReportJSON {
			continue
		}
		if rel := FindPatternFile(dir, f); rel != "" {
			files = append(files, rel)
		}
	}

	return kind, spec, files, nil
}

// LoadPatternMeta reads name/version/description from the primary file.
// Works for both Blueprint (blueprint.yaml) and Module (module.yaml).
func LoadPatternMeta(dir string, spec *PatternSpec) (*PatternMeta, error) {
	path := filepath.Join(dir, spec.PrimaryFile)
	data, err := readLocal(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", spec.PrimaryFile, err)
	}
	var raw struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name        string   `yaml:"name"`
			Version     string   `yaml:"version"`
			Description string   `yaml:"description"`
			Author      string   `yaml:"author"`
			License     string   `yaml:"license"`
			Tags        []string `yaml:"tags"`
		} `yaml:"metadata"`
		Lifecycle *types.BlueprintLifecycle `yaml:"lifecycle"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", spec.PrimaryFile, err)
	}
	if raw.Metadata.Name == "" {
		return nil, fmt.Errorf("%s: metadata.name is required", spec.PrimaryFile)
	}
	meta := &PatternMeta{
		Kind:        PatternKind(raw.Kind),
		Name:        raw.Metadata.Name,
		Version:     raw.Metadata.Version,
		Description: raw.Metadata.Description,
		Author:      raw.Metadata.Author,
		License:     raw.Metadata.License,
		Tags:        raw.Metadata.Tags,
	}
	if raw.Lifecycle != nil {
		if d := raw.Lifecycle.Deprecation; d != nil {
			meta.Deprecated = &PatternDeprecated{
				MigratedTo:   d.MigratedTo,
				Message:      d.Message,
				TimelineFrom: d.TimelineFrom(),
				TimelineTo:   d.TimelineTo(),
			}
		}
	}
	if meta.Version == "" {
		meta.Version = "latest"
	}
	if meta.Description == "" {
		meta.Description = fmt.Sprintf("%s %s", kind(spec.Kind), meta.Name)
	}
	return meta, nil
}

// kind returns a display string for a PatternKind.
func kind(k PatternKind) string {
	switch k {
	case BlueprintKind:
		return "Pattern"
	case ModuleKind:
		return "Module"
	default:
		return "Pattern"
	}
}

// mediaTypeForPatternFile returns the OCI layer media type for a file within
// a specific pattern kind.
func mediaTypeForPatternFile(name string, k PatternKind) string {
	switch filepath.Base(name) {
	case FileBlueprint:
		return "application/vnd.inrun.blueprint.v1+yaml"
	case FileCRD:
		return "application/vnd.kubernetes.crd.v1+yaml"
	case FileCR:
		return "application/vnd.kubernetes.cr.v1+yaml"
	case FileReadme:
		return "text/markdown"
	case FileModule:
		return "application/vnd.inrun.module.v1+yaml"
	case FileE2E:
		return "application/vnd.inrun.e2e.v1+yaml"
	case FileSimulate:
		return "application/vnd.inrun.simulate.v1+yaml"
	case FileReportJSON:
		return "application/vnd.inrun.report.v1+json"
	case FileGoMod, FileGoSum:
		return "text/plain"
	case FileMakefile:
		return "text/x-makefile"
	default:
		return "application/octet-stream"
	}
}

// groupDir is the directory a pattern file may live in besides the root:
// what is applied to the cluster in manifests/, simulate and e2e specs in test/.
var groupDir = map[string]string{
	FileCRD:      DirManifests,
	FileCR:       DirManifests,
	FileE2E:      DirTest,
	FileSimulate: DirTest,
}

// PatternFilePaths returns the relative paths name may have in a pattern:
// the root, then its group directory.
func PatternFilePaths(name string) []string {
	if d, ok := groupDir[name]; ok {
		return []string{name, filepath.Join(d, name)}
	}
	return []string{name}
}

// FindPatternFile returns the relative path of name within dir, or "" when
// the pattern does not have it.
func FindPatternFile(dir, name string) string {
	return ResolvePatternFile(name, func(rel string) bool {
		_, err := os.Stat(filepath.Join(dir, rel))
		return err == nil
	})
}

// ResolvePatternFile returns the first of name's paths in a pattern (see
// PatternFilePaths) for which exists reports true, or "". FindPatternFile
// checks a directory; callers holding a pattern's file list check that.
func ResolvePatternFile(name string, exists func(rel string) bool) string {
	for _, rel := range PatternFilePaths(name) {
		if exists(rel) {
			return rel
		}
	}
	return ""
}
