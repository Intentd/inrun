package e2e

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// IsE2EDoc returns true when the file's kind is "E2E".
func IsE2EDoc(path string) bool {
	data, err := readLocal(path)
	if err != nil {
		return false
	}
	var head struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &head); err != nil {
		return false
	}
	return head.Kind == config.E2EKind()
}

// ValidateImports checks that every file listed in imports exists and declares
// kind: E2E. baseDir is the directory that relative paths are resolved against.
// Returns one error per invalid import — callers may print all of them.
func ValidateImports(baseDir string, imports []types.E2EImport) []error {
	var errs []error
	for _, imp := range imports {
		path := imp.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		data, err := readLocal(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", imp.Path, err))
			continue
		}
		var head struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal(data, &head); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", imp.Path, err))
			continue
		}
		if head.Kind != "E2E" {
			errs = append(errs, fmt.Errorf("%s: expected kind E2E, got %q", imp.Path, head.Kind))
		}
		if imp.Wait != "" {
			if _, err := parseTimeDuration(imp.Wait); err != nil {
				errs = append(errs, fmt.Errorf("%s: wait %q is not a valid duration: %w", imp.Path, imp.Wait, err))
			}
		}
	}
	return errs
}

// ValidateCluster checks the spec's cluster configuration and the
// expectations that depend on it. Returns one error per violation.
//
// Exported so `inrun validate` can run it before `inrun e2e` provisions
// anything.
func ValidateCluster(spec *types.E2ESpec) []error {
	if spec == nil {
		return nil
	}

	var errs []error
	provider := spec.Cluster.EffectiveProvider()

	if !provider.IsValid() {
		return []error{fmt.Errorf(
			"cluster.provider %q is not supported — use one of: %s",
			spec.Cluster.Provider, strings.Join(types.ValidClusterProviders(), ", "))}
	}

	if provider == types.ProviderNone {
		if spec.Blueprint != "" || spec.CRD != "" || len(spec.CRDFiles) > 0 || len(spec.CRFiles) > 0 {
			errs = append(errs, fmt.Errorf(
				"cluster.provider none cannot install an operator — omit blueprint, crd, crdFiles, and crFiles"))
		}
		for i, exp := range spec.Expect {
			switch {
			case exp.Kubectl != nil:
				errs = append(errs, fmt.Errorf(
					"expect[%d] %q: kubectl requires a cluster; cluster.provider is none", i, exp.Name))
			case len(exp.Resources) > 0:
				errs = append(errs, fmt.Errorf(
					"expect[%d] %q: resources requires a cluster; cluster.provider is none", i, exp.Name))
			case len(exp.External) > 0:
				errs = append(errs, fmt.Errorf(
					"expect[%d] %q: external requires a cluster; cluster.provider is none", i, exp.Name))
			}
		}
	}

	return errs
}
