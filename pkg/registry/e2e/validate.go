package e2e

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kel/ecosystem"
	"github.com/intentd/tools/fileio"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/validation"
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
	provider := spec.Environment.Provider()

	if !provider.IsValid() {
		return []error{fmt.Errorf(
			"cluster.provider %q is not supported — use one of: %s",
			spec.Environment.Cluster.Provider, strings.Join(types.ValidClusterProviders(), ", "))}
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

// ValidatePaths reports every local file the spec names that is not there,
// resolved from baseDir, the suite's directory, as the run resolves them.
// URLs, templated paths and repository charts are left to the run.
//
// Exported so `inrun validate` catches a missing file before `inrun e2e`
// provisions anything.
func ValidatePaths(baseDir string, spec *types.E2ESpec) []error {
	var errs []error
	check := func(field, ref string) {
		path, ok := fileio.LocalRef(baseDir, ref)
		if !ok {
			return
		}
		if err := fileio.CheckLocal(path); err != nil {
			errs = append(errs, fmt.Errorf("spec.%s %q: %w", field, ref, err))
		}
	}

	check("blueprint", spec.Blueprint)
	check("crd", spec.CRD)
	for i, p := range spec.CRDFiles {
		check(fmt.Sprintf("crdFiles[%d]", i), p)
	}
	check("cr", spec.CR)
	for i, p := range spec.CRFiles {
		check(fmt.Sprintf("crFiles[%d]", i), p)
	}
	for i, p := range spec.ValuesFiles {
		check(fmt.Sprintf("valuesFiles[%d]", i), p)
	}
	if e := spec.Environment; e != nil {
		for i, img := range e.Images {
			if img.Build != nil {
				// The dockerfile is relative to the context, as in Compose.
				b := img.Build.ResolvePaths(baseDir)
				check(fmt.Sprintf("environment.images[%d].build.context", i), img.Build.Context)
				check(fmt.Sprintf("environment.images[%d].build.dockerfile", i), b.Dockerfile)
			}
		}
		for i, a := range e.Apply {
			check(fmt.Sprintf("environment.apply[%d].path", i), a.Path)
		}
		for i, h := range e.Helm {
			// The run decides the same way which charts are local paths.
			if h.IsLocalChart() {
				check(fmt.Sprintf("environment.helm[%d].chart", i), h.Chart)
			}
			for j, f := range h.ValueFiles {
				check(fmt.Sprintf("environment.helm[%d].valueFiles[%d]", i, j), f)
			}
		}
	}
	if p := spec.Prepare; p != nil {
		for i, a := range p.Apply {
			check(fmt.Sprintf("prepare.apply[%d].path", i), a.Path)
		}
		for i, h := range p.Helm {
			// The run decides the same way which charts are local paths.
			if h.IsLocalChart() {
				check(fmt.Sprintf("prepare.helm[%d].chart", i), h.Chart)
			}
			for j, f := range h.ValueFiles {
				check(fmt.Sprintf("prepare.helm[%d].valueFiles[%d]", i, j), f)
			}
		}
	}
	return errs
}

// ValidateImages validates image references and deployment configuration
// declared in the environment.
//
// Build contexts and Dockerfiles are checked separately by ValidatePaths.
func ValidateImages(spec *types.E2ESpec) []error {
	if spec == nil || spec.Environment == nil {
		return nil
	}

	var errs []error
	deployments := make(map[string]int)

	for i, img := range spec.Environment.Images {
		prefix := fmt.Sprintf("spec.environment.images[%d]", i)

		if img.Image != "" {
			if !ecosystem.IsValidImageRef(img.Image) {
				errs = append(errs, fmt.Errorf("%s.image %q is invalid", prefix, img.Image))
			}
		} else if img.Build == nil {
			errs = append(errs, fmt.Errorf("%s requires image or build", prefix))
		}

		if img.Deploy == nil {
			continue
		}

		d := img.Deploy

		name := d.Name
		if name == "" {
			name = img.DefaultDeployName()
		}

		namespace := d.Namespace
		if namespace == "" {
			namespace = "default"
		}

		if name != "" {
			if problems := validation.IsDNS1123Label(name); len(problems) > 0 {
				errs = append(errs, fmt.Errorf(
					"%s.deploy.name %q is invalid: %s",
					prefix, name, strings.Join(problems, ", "),
				))
			}

			key := namespace + "/" + name
			if previous, exists := deployments[key]; exists {
				errs = append(errs, fmt.Errorf(
					"%s.deploy duplicates deployment %q from images[%d]",
					prefix, key, previous,
				))
			} else {
				deployments[key] = i
			}
		}

		if problems := validation.IsDNS1123Label(namespace); len(problems) > 0 {
			errs = append(errs, fmt.Errorf(
				"%s.deploy.namespace %q is invalid: %s",
				prefix, namespace, strings.Join(problems, ", "),
			))
		}

		if d.Port < 1 || d.Port > 65535 {
			errs = append(errs, fmt.Errorf(
				"%s.deploy.port must be between 1 and 65535",
				prefix,
			))
		}

		if d.Replicas != nil && *d.Replicas < 0 {
			errs = append(errs, fmt.Errorf(
				"%s.deploy.replicas cannot be negative",
				prefix,
			))
		}
	}

	return errs
}
