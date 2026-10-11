package types

// Simulate is the schema for simulate.yaml.
//
// Without expect: it runs in op-print mode.
// With expect: it asserts that the reconciler produces the declared ops and
// reaches the declared steady state — the run passes or fails like a test.
//
// Aggregator form (imports + no spec) runs each imported simulate.yaml as an
// independent test, reporting pass/fail for each.
type Simulate struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   SimulateMeta  `yaml:"metadata"`
	Imports    []string      `yaml:"imports,omitempty"`
	Spec       *SimulateSpec `yaml:"spec,omitempty"`
}

type SimulateMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

type SimulateSpec struct {
	Catalog string   `yaml:"catalog"`
	CR      string   `yaml:"cr,omitempty"`
	CRFiles []string `yaml:"crFiles,omitempty"`
	// Intent is a serve intent file (JSON or YAML) to build the CR from, the
	// way the gateway and inrun serve play do. Used instead of cr/crFiles.
	Intent string `yaml:"intent,omitempty"`
	// Token is the gateway token the intent is sent with; required with Intent.
	Token        string          `yaml:"token,omitempty"`
	CRD          string          `yaml:"crd,omitempty"`
	CRDFiles     []string        `yaml:"crdFiles,omitempty"`
	Cycles       int             `yaml:"cycles,omitempty"` // default 10 when unset
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

// SimulateExpect declares what the reconciler must produce.
// Top-level fields apply to all CRDs; crds overrides per named CRD.
type SimulateExpect struct {
	Steady   *bool                      `yaml:"steady,omitempty"`
	SteadyAt *int                       `yaml:"steadyAt,omitempty"` // result.SteadyAt must be ≤ this
	NoErrors bool                       `yaml:"noErrors,omitempty"`
	Ops      []SimulateOpRule           `yaml:"ops,omitempty"`
	Absent   []SimulateOpRule           `yaml:"absent,omitempty"` // ops that must NOT appear
	CRDs     map[string]*SimulateExpect `yaml:"crds,omitempty"`
}

// SimulateOpRule asserts that at least one recorded op in the given cycle
// matches all non-empty fields. count overrides the minimum (default ≥1).
// When include is set the entry is replaced by the ops: list in the referenced
// file; all other fields must be empty in that case.
type SimulateOpRule struct {
	Include  string `yaml:"include,omitempty"` // path to a file with an ops: list
	Cycle    int    `yaml:"cycle"`
	Verb     string `yaml:"verb"`            // create | update | delete | patch
	Resource string `yaml:"resource"`        // deployments | statefulsets | etc.
	Name     string `yaml:"name,omitempty"`  // optional: match a specific resource name
	Count    int    `yaml:"count,omitempty"` // 0 = at least 1
}
