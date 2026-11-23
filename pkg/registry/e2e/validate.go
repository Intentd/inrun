package e2e

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/jig"
	"github.com/intentd/tools/fileio"
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

// noClusterInstall reports an operator a test without a cluster declares:
// with cluster.provider none there is nowhere to install it. What else such
// a test can't do, jig checks.
func noClusterInstall(spec *types.E2ESpec) []error {
	if spec == nil || spec.Environment.Provider() != types.ProviderNone {
		return nil
	}
	if spec.Blueprint != "" || spec.CRD != "" || len(spec.CRDFiles) > 0 || len(spec.CRFiles) > 0 {
		return []error{fmt.Errorf("cluster.provider none cannot install an operator — omit blueprint, crd, crdFiles, and crFiles")}
	}
	return nil
}

// ValidatePaths reports every file Inrun's own fields name that is not
// there (blueprint, crd and crdFiles, cr and crFiles, valuesFiles), resolved
// from baseDir as the run resolves them. jig checks the rest.
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
	return errs
}

// Validate checks the Inrun e2e test at path, offline: Inrun's own fields
// first (the name, a crd with a blueprint, what a test without a cluster
// can't install, the files they name, the imports), then the test mapped to
// jig (Map), checked by jig. It returns the mapped test, for jig.Plan, and
// every problem; the test is nil when Inrun's own fields stop the mapping.
func Validate(path string) (*jig.Declaration, []error) {
	data, err := readLocal(path)
	if err != nil {
		return nil, []error{err}
	}
	var doc types.E2E
	if err := strictUnmarshal(data, &doc); err != nil {
		return nil, []error{fmt.Errorf("parsing %s: %w", path, err)}
	}
	baseDir := filepath.Dir(path)
	var errs []error
	if doc.Metadata.Name == "" {
		errs = append(errs, fmt.Errorf("metadata.name is required"))
	}
	noCluster := doc.Spec.Environment.Provider() == types.ProviderNone
	if doc.Spec.Blueprint != "" && doc.Spec.CRD == "" && !noCluster {
		errs = append(errs, fmt.Errorf("spec.crd is required with spec.blueprint"))
	}
	aggregator := len(doc.Imports) > 0 && doc.Spec.Blueprint == "" && doc.Spec.Init == nil
	if !aggregator && len(doc.Spec.Expect) == 0 {
		errs = append(errs, fmt.Errorf("spec.expect must contain at least one expectation"))
	}
	errs = append(errs, noClusterInstall(&doc.Spec)...)
	errs = append(errs, ValidatePaths(baseDir, &doc.Spec)...)
	for _, e := range ValidateImports(baseDir, doc.Imports) {
		errs = append(errs, fmt.Errorf("import: %w", e))
	}
	if len(errs) > 0 {
		return nil, errs
	}

	r, err := New(path, Options{})
	if err != nil {
		return nil, []error{err}
	}
	d, err := r.Map()
	if err != nil {
		return nil, []error{err}
	}
	return d, jig.ValidateWith(d, funcs.WithKEL())
}
