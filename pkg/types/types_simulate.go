package types

import "fmt"

// Simulate is the schema for simulate.yaml.
//
// Without expect: it runs in op-print mode.
// With expect: it asserts that the reconciler produces the declared ops and
// reaches the declared steady state — the run passes or fails like a test.
//
// Aggregator form (imports + no spec) runs each imported simulate.yaml as an
// independent test, reporting pass/fail for each.
type Simulate struct {
	// APIVersion — inrun.dev/v1.
	APIVersion string `yaml:"apiVersion"`
	// Kind — Simulate.
	Kind string `yaml:"kind"`
	// Metadata — the simulation's name and description.
	Metadata SimulateMeta `yaml:"metadata"`
	// Imports — other simulate files to run, each as its own test. A file
	// with imports and no spec only runs those.
	Imports []string `yaml:"imports,omitempty"`
	// Spec — the Blueprint, the CR and what the reconcile must produce.
	Spec *SimulateSpec `yaml:"spec,omitempty"`
}

type SimulateMeta struct {
	// Name — shown in the results.
	Name string `yaml:"name"`
	// Description — what the simulation checks.
	Description string `yaml:"description,omitempty"`
}

type SimulateSpec struct {
	// Blueprint — the Blueprint (or Stack) file, relative to this one.
	Blueprint string `yaml:"blueprint"`
	// CR — the CR to reconcile, a file relative to this one.
	CR string `yaml:"cr,omitempty"`
	// CRFiles — several CR files, reconciled together.
	CRFiles []string `yaml:"crFiles,omitempty"`
	// Intent is a serve intent file (JSON or YAML) to build the CR from, the
	// way the gateway and inrun serve play do. Used instead of cr/crFiles.
	Intent string `yaml:"intent,omitempty"`
	// Token is the gateway token the intent is sent with; required with Intent.
	Token string `yaml:"token,omitempty"`
	// CRD — a CRD manifest for --envtest to install. Default: the Blueprint's
	// crdFile entries.
	CRD string `yaml:"crd,omitempty"`
	// CRDFiles — several CRD manifests for --envtest.
	CRDFiles []string `yaml:"crdFiles,omitempty"`
	// Setup — manifests applied before the CR, after the Blueprint's own
	// setup: what the test needs in the cluster, such as the CR's
	// Namespace. Paths relative to this file, as a list or setup.apply;
	// simulate applies them only, so helm and wait are rejected.
	Setup  *SetupConfig `yaml:"setup,omitempty"`
	Cycles int          `yaml:"cycles,omitempty"` // default 10 when unset
	// SkipExternal — answer external calls with an empty 200 instead of
	// making them.
	SkipExternal bool            `yaml:"skipExternal,omitempty"`
	Target       string          `yaml:"target,omitempty"` // serve target to use for reconciliation
	Expect       *SimulateExpect `yaml:"expect,omitempty"` // nil = op-print only
}

// AllCRPaths returns all CR file paths in declaration order (cr then crFiles).
func (s *SimulateSpec) AllCRPaths() []string {
	var paths []string
	if s.CR != "" {
		paths = append(paths, s.CR)
	}
	paths = append(paths, s.CRFiles...)
	return paths
}

// AllCRDPaths returns all CRD file paths in declaration order (crd then crdFiles).
func (s *SimulateSpec) AllCRDPaths() []string {
	var paths []string
	if s.CRD != "" {
		paths = append(paths, s.CRD)
	}
	paths = append(paths, s.CRDFiles...)
	return paths
}

// SetupPaths returns the setup.apply manifest paths, as written.
func (s *SimulateSpec) SetupPaths() []string {
	if s.Setup == nil {
		return nil
	}
	paths := make([]string, 0, len(s.Setup.Apply))
	for _, e := range s.Setup.Apply {
		paths = append(paths, e.Path)
	}
	return paths
}

// ValidateSetup reports what simulate cannot do with setup: it applies
// manifests and nothing else, so helm, wait and an apply entry's wait are
// errors.
func (s *SimulateSpec) ValidateSetup() []string {
	if s.Setup == nil {
		return nil
	}
	var errs []string
	if len(s.Setup.Helm) > 0 {
		errs = append(errs, "spec.setup.helm: simulate applies manifests only; install charts in an e2e test")
	}
	if len(s.Setup.Wait) > 0 {
		errs = append(errs, "spec.setup.wait: simulate applies manifests only; there is nothing to wait for")
	}
	for i, e := range s.Setup.Apply {
		if len(e.Wait) > 0 {
			errs = append(errs, fmt.Sprintf("spec.setup.apply[%d].wait: simulate applies manifests only; there is nothing to wait for", i))
		}
	}
	return errs
}

// SimulateExpect declares what the reconciler must produce.
// Top-level fields apply to all CRDs; crds overrides per named CRD.
type SimulateExpect struct {
	// Steady — true: the reconcile must settle, a cycle after the first making
	// no writes or repeating the previous cycle's.
	Steady   *bool `yaml:"steady,omitempty"`
	SteadyAt *int  `yaml:"steadyAt,omitempty"` // result.SteadyAt must be ≤ this
	// NoErrors — no reconcile may return an error.
	NoErrors bool `yaml:"noErrors,omitempty"`
	// Ops — writes that must appear.
	Ops    []SimulateOpRule `yaml:"ops,omitempty"`
	Absent []SimulateOpRule `yaml:"absent,omitempty"` // ops that must NOT appear
	// CRDs — expectations for one CRD of a multi-CRD Blueprint, by name.
	CRDs map[string]*SimulateExpect `yaml:"crds,omitempty"`
}

// SimulateOpRule asserts that at least one recorded op in the given cycle
// matches all non-empty fields. count overrides the minimum (default ≥1).
// When include is set the entry is replaced by the ops: list in the referenced
// file; all other fields must be empty in that case.
type SimulateOpRule struct {
	Include string `yaml:"include,omitempty"` // path to a file with an ops: list
	// Cycle — the reconcile cycle the op must appear in, from 1.
	Cycle    int    `yaml:"cycle"`
	Verb     string `yaml:"verb"`            // apply | create | update | patch | delete
	Resource string `yaml:"resource"`        // deployments | statefulsets | etc.
	Name     string `yaml:"name,omitempty"`  // optional: match a specific resource name
	Count    int    `yaml:"count,omitempty"` // 0 = at least 1
}
