package types

import (
	"fmt"
	"slices"
	"strings"
)

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
	SkipExternal bool   `yaml:"skipExternal,omitempty"`
	Target       string `yaml:"target,omitempty"` // serve target to use for reconciliation

	// Functions declares user-defined functions available in when:/or: expressions.
	// Same syntax as a Blueprint functions block — same FuncMap registration, same functions.
	Functions KELFunctionRegistry `yaml:"functions,omitempty"`

	Expect *SimulateExpect `yaml:"expect,omitempty"` // nil = op-print only
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
	// When — all conditions must pass for this expectation to run (AND).
	// Empty means unconditional. Functions declared in spec.functions are available as
	// template expressions: field: '{{ inBusinessHours }}'.
	When Conditions `yaml:"when,omitempty"`
	// Or — at least one condition must pass, alongside When if declared..
	// Functions declared in spec.functions are available as
	// template expressions: field: '{{ inBusinessHours }}'.
	Or Conditions `yaml:"or,omitempty"`

	// Steady — true: the reconcile must settle, a cycle after the first making
	// no writes or repeating the previous cycle's.
	Steady   *bool `yaml:"steady,omitempty"`
	SteadyAt *int  `yaml:"steadyAt,omitempty"` // result.SteadyAt must be ≤ this
	// NoErrors — no reconcile may return an error.
	NoErrors bool `yaml:"noErrors,omitempty"`
	// Ops — writes that must appear.
	Ops    SimulateOpRules `yaml:"ops,omitempty"`
	Absent SimulateOpRules `yaml:"absent,omitempty"` // ops that must NOT appear
	// CRDs — expectations for one CRD of a multi-CRD Blueprint, by name.
	CRDs map[string]*SimulateExpect `yaml:"crds,omitempty"`
}

// ExpectForCRD returns the expect block for a named CRD. An explicit entry in
// expect.crds wins; an entry with no body falls through to the top-level default.
func (s *SimulateSpec) ExpectForCRD(crdName string) *SimulateExpect {
	if s == nil || s.Expect == nil {
		return nil
	}
	if e, ok := s.Expect.CRDs[crdName]; ok && e != nil {
		return e
	}
	return s.Expect
}

// WriteVerbs are the verbs of a recorded op that change the cluster — what
// simulate prints, asserts with ops: and absent:, compares and reports.
// Reads (get, list, watch) are recorded too, but are none of these.
var WriteVerbs = []string{"create", "apply", "update", "patch", "delete"}

// IsWriteVerb reports whether verb is one of WriteVerbs.
func IsWriteVerb(verb string) bool {
	return slices.Contains(WriteVerbs, verb)
}

// invalidVerb is the message for an op rule's verb that is not a write.
func invalidVerb(path string, i int, verb string) string {
	return fmt.Sprintf("%s[%d]: invalid verb %q (must be %s, or %s)", path, i, verb,
		strings.Join(WriteVerbs[:len(WriteVerbs)-1], ", "), WriteVerbs[len(WriteVerbs)-1])
}

func ValidateOpsRules(ops, absent SimulateOpRules, prefix string) (errs []string) {

	validateOpRules := func(rules SimulateOpRules, path string) {
		for i, rule := range rules {
			hasVerb := rule.Verb != ""
			hasResource := rule.Resource != ""
			hasAssert := rule.Assert != nil

			switch {
			case !hasVerb && !hasResource && !hasAssert:
				errs = append(errs, fmt.Sprintf("%s[%d]: verb and resource, or assert, is required", path, i))
			case hasVerb != hasResource:
				errs = append(errs, fmt.Sprintf("%s[%d]: verb and resource must both be set", path, i))
			case hasVerb && !IsWriteVerb(rule.Verb):
				errs = append(errs, invalidVerb(path, i, rule.Verb))
			}

			if hasAssert && rule.Assert.HasJSON() {
				errs = append(errs, fmt.Sprintf("%s[%d].assert: json: is not supported in simulate (cycle output is plain text)", path, i))
			}
		}
	}

	validateAbsentRules := func(rules SimulateOpRules, path string) {
		for i, rule := range rules {
			switch {
			case rule.Resource == "":
				errs = append(errs, fmt.Sprintf("%s[%d]: resource is required", path, i))
			case rule.Verb != "" && !IsWriteVerb(rule.Verb):
				errs = append(errs, invalidVerb(path, i, rule.Verb))
			}
		}
	}

	validateOpRules(ops, prefix+".ops")
	validateAbsentRules(absent, prefix+".absent")

	return errs
}

// ValidateExpect validates an expect block and is useful for include file validation.
func (s *SimulateSpec) ValidateExpect(baseDir string) (errs []string) {
	if s == nil || s.Expect == nil {
		return nil
	}

	e := s.Expect
	if expandErr := ExpandSimulateOpsAndAbsentIncludes(e, baseDir); expandErr != nil {
		errs = append(errs, "expanding ops includes: "+expandErr.Error())
	}

	errs = append(errs, ValidateOpsRules(e.Ops, e.Absent, "expect")...)

	for crdName, sub := range e.CRDs {
		if sub == nil {
			continue
		}
		errs = append(errs, ValidateOpsRules(
			sub.Ops,
			sub.Absent,
			fmt.Sprintf("expect.crds[%s]", crdName),
		)...)
	}

	return errs
}

// SimulateOpRule asserts that at least one recorded op in the given cycle
// matches all non-empty fields. count overrides the minimum (default ≥1).
// When include is set the entry is replaced by the ops: list in the referenced
// file; all other fields must be empty in that case.
type SimulateOpRule struct {
	Include string `yaml:"include,omitempty"` // path to a file with an ops: list
	// Cycle — the reconcile cycle the op must appear in, from 1.
	Cycle    int    `yaml:"cycle"`
	Verb     string `yaml:"verb"`           // apply | create | update | patch | delete
	Resource string `yaml:"resource"`       // deployments | statefulsets | etc.
	Name     string `yaml:"name,omitempty"` // optional: match a specific resource name
	// Namespace — optional: match the namespace the op wrote in, e.g. each
	// namespace a cross-namespace copy writes to.
	Namespace string `yaml:"namespace,omitempty"`
	Count     int    `yaml:"count,omitempty"` // 0 = at least 1
	// Assert checks the output with the full block: json and jsonPath are not supported
	// since the result is almost always a text
	Assert *Assertions `yaml:"assert,omitempty"`
	// TextAssertion shorthands — equals:, contains:, prefix:, … — applied to
	// the output directly, without nesting under assert:.
	TextAssertion `yaml:",inline"`
	// When — all conditions must pass for this rule to run (AND).
	// Empty means unconditional. Functions declared in spec.functions are available as
	// template expressions: field: '{{ inBusinessHours }}'.
	When Conditions `yaml:"when,omitempty"`
	// Or — at least one condition must pass, alongside When if declared. if declared.
	// Functions declared in spec.functions are available as
	// template expressions: field: '{{ inBusinessHours }}'.
	Or Conditions `yaml:"or,omitempty"`
}

type SimulateOpRules []SimulateOpRule
