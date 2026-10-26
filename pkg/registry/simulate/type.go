package simulate

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Simulator holds the inputs for one simulation run. Obtain one via New;
// call Run or RunWithEnvtest to execute.
type Simulator struct {
	ctx  context.Context
	opts RunOptions
}

// RunOptions controls what to simulate and how.
type RunOptions struct {
	// Blueprint — the parsed Blueprint to simulate against.
	Blueprint *blueprint.Blueprint
	// CRDName — the CRD entry in the Blueprint to reconcile.
	CRDName string
	// CR — the CR to reconcile.
	CR *unstructured.Unstructured
	// MaxCycles — maximum reconcile cycles to run (default 10 when zero).
	MaxCycles int

	// SkipExternal stubs all external: HTTP calls with an empty 200 response.
	// When false (the default), external calls are attempted against the real network.
	SkipExternal bool

	// Peers holds CRs for sibling CRDs in the same Blueprint, keyed by lowercase kind.
	// When set, cross: declarations in the reconciler can observe these CRs
	// via the fake blueprint registry instead of returning empty results.
	Peers map[string]*unstructured.Unstructured

	// ExistingInstances holds other, already-existing instances of the SAME
	// CRD being simulated — from extra documents of the CR's own kind in a
	// multi-doc CR file. They are seeded into the fake dynamic client (so
	// reconcile-time checks that list other instances, like operator:
	// unique, can see them) but are never themselves reconciled.
	ExistingInstances []*unstructured.Unstructured

	// Target specifies the target to use for this simulation
	Target string

	// Setup holds manifest paths applied after the CRD's own setup and
	// before the CR: the simulate spec's setup.
	Setup []string

	// Verbose keeps the reconciler's logs, which are otherwise discarded.
	Verbose bool

	// CRDPaths — CRD manifest paths for envtest to install (envtest only).
	CRDPaths []string
	// K8sVersion — Kubernetes version for envtest binaries (envtest only).
	K8sVersion string
}

// Isolated returns opts with its own copies of the CR and the objects
// seeded beside it, so a run cannot change what another starts from.
func (o RunOptions) Isolated() RunOptions {
	cp := o
	if o.CR != nil {
		cp.CR = o.CR.DeepCopy()
	}
	if o.Peers != nil {
		cp.Peers = make(map[string]*unstructured.Unstructured, len(o.Peers))
		for k, v := range o.Peers {
			cp.Peers[k] = v.DeepCopy()
		}
	}
	cp.ExistingInstances = nil
	for _, u := range o.ExistingInstances {
		cp.ExistingInstances = append(cp.ExistingInstances, u.DeepCopy())
	}
	return cp
}

// Result is the output of one simulation run.
type Result struct {
	Cycles   []CycleResult
	Steady   bool
	SteadyAt int      // cycle number where steady state was first detected (0 if not reached)
	Notes    []string // informational notes about blocks that could not execute (e.g. constructor body)
	AllOps   Ops      // every op recorded across all cycles, for diagnostic use
	Name     string
	Elapsed  time.Duration
	// Startup — time spent starting the envtest API server, not reconciling.
	Startup time.Duration
	// Final — the CR and every object it wrote, read back after the last
	// cycle: what the cluster holds, for comparing backends.
	Final []*unstructured.Unstructured
	// RandomSecretKeys — the data keys of the CRD's declared Secrets filled
	// from a random function: values that differ between runs by design.
	RandomSecretKeys []string
	Assertions       AssertionSummary
	Imports          []ImportResult
	// Comparisons — with --compare, one per CRD simulated: the in-memory
	// run against the envtest run.
	Comparisons []*Comparison

	// folded counts the results Add has folded in, so the first sets Steady.
	folded int
}

// ImportResult holds the outcome of one imported simulate file in an aggregator run.
type ImportResult struct {
	// Path is the import path as declared in imports:.
	Path string
	// Result is the structured outcome. Nil when the import failed to load.
	Result *Result
	// Err is non-nil when the import failed (load error or assertion failures).
	Err error
}

// Add folds other's assertions and comparisons into r: a CRD's run into its
// file's result, or a file's into a suite's. r is steady when every result
// folded in is, at the latest of their steady cycles. A nil other adds
// nothing.
func (r *Result) Add(other *Result) {
	if other == nil {
		return
	}
	if r.folded == 0 {
		r.Steady = other.Steady
	} else {
		r.Steady = r.Steady && other.Steady
	}
	r.SteadyAt = max(r.SteadyAt, other.SteadyAt)
	if !r.Steady {
		r.SteadyAt = 0
	}
	r.folded++
	r.Assertions.Total += other.Assertions.Total
	r.Assertions.Passed += other.Assertions.Passed
	r.Assertions.Failed += other.Assertions.Failed
	r.Assertions.Skipped += other.Assertions.Skipped
	r.Assertions.Cases = append(r.Assertions.Cases, other.Assertions.Cases...)
	r.Comparisons = append(r.Comparisons, other.Comparisons...)
}

// AddImport records one file run as part of r — an imports: entry, or a
// file ./... discovered — and folds its result in. res is nil when the file
// failed to load; err is why it failed.
func (r *Result) AddImport(path string, res *Result, err error) {
	r.Imports = append(r.Imports, ImportResult{Path: path, Result: res, Err: err})
	r.Add(res)
}

// WithImports returns true when the result aggregates imported suites.
func (r *Result) WithImports() bool {
	return r != nil && len(r.Imports) > 0
}

type AssertionSummary struct {
	Status  registry.Status
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Cases   []CaseResult
}

type CaseResult struct {
	Field   string
	Err     error
	Skipped bool
}

// CycleResult is the output of one reconcile cycle.
type CycleResult struct {
	Cycle int
	Ops   Ops
	Error error
}

// AllPassed reports whether every case passed.
func (r *Result) AllPassed() bool {
	return r != nil && r.Assertions.Failed == 0
}

// Passed returns the number of passing cases.
func (r *Result) Passed() int {
	if r == nil {
		return 0
	}
	return r.Assertions.Passed
}

// Failed returns the number of failing cases.
func (r *Result) Failed() int {
	if r == nil {
		return 0
	}
	return r.Assertions.Failed
}

// Skipped returns the number of skipped cases.
func (r *Result) Skipped() int {
	if r == nil {
		return 0
	}
	return r.Assertions.Skipped
}

// Total returns the total number of cases.
func (r *Result) Total() int {
	if r == nil {
		return 0
	}
	return r.Assertions.Total
}

// Duration returns the total elapsed time as a human-readable string (e.g. "45s").
func (r *Result) Duration() string {
	if r == nil {
		return "0s"
	}
	return r.Elapsed.Round(time.Second).String()
}

// SimulateStatusFor derives the gate status from an aggregated result.
func (r *Result) SimulateStatusFor() registry.Status {
	switch {
	case r.Assertions.Failed > 0:
		return registry.StatusFailed
	case r.Assertions.Total == 0:
		return registry.StatusNoAssertion
	default:
		return registry.StatusPassed
	}
}
