package testsuite

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// readSpecCRs reads a spec's CR files, relative to dir, grouped by kind.
func readSpecCRs(dir string, paths []string) (map[string][]*unstructured.Unstructured, error) {
	var buf []byte
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		data, err := cmdutil.ReadLocal(p)
		if err != nil {
			return nil, fmt.Errorf("reading CR %s: %w", p, err)
		}
		if len(buf) > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, data...)
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("spec.cr, spec.crFiles or spec.intent is required")
	}
	crs := cmdutil.ParseMultiDocCRs(buf)
	if len(crs) == 0 {
		return nil, fmt.Errorf("no valid CR documents in CR file(s)")
	}
	return crs, nil
}

// PlayIntent builds a CR from an intent file the way `inrun serve play` does,
// returning the constructed object. The serve package owns the implementation
// and assigns this in init, because that package owns the stage chain.
//
// With Verbose set every stage is printed; without it nothing is, and an error
// is returned for the caller to report. The CR is built the same either way.
//
// Set once during init and read-only thereafter.
var PlayIntent func(PlayRequest) (*unstructured.Unstructured, error)

type PlayRequest struct {
	Catalog     *catalog.Catalog     // the catalog the intent is resolved against
	CatalogFile string               // path to the catalog.yaml
	IntentFile  string               // path to the intent YAML/JSON
	Token       string               // serve token to check the op against
	Op          types.ServeOperation // create / update / delete
	Cycles      int                  // reconcile cycles for simulate
	Verbose     bool                 // print each stage
}

// CatalogCRDPaths returns the CRD files the Catalog's CRDs point at with
// crdFile, resolved like the Catalog parser does. --envtest installs them when
// a Simulate spec lists no crd or crdFiles.
func CatalogCRDPaths(m *merger.Merger) []string {
	dir := m.FirstEntryDir()
	seen := map[string]bool{}
	var paths []string
	for _, crd := range m.Enabled() {
		p := crd.CRDFile
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func printAssertions(errs []simulate.AssertionError, expect *types.SimulateExpect) {
	failSet := map[string]bool{}
	for _, e := range errs {
		failSet[e.Field] = true
	}

	if expect.Steady != nil {
		if failSet["steady"] {
			fmt.Printf("  %s steady\n", cmdutil.FailureMark())
		} else {
			fmt.Printf("  %s steady\n", cmdutil.SuccessMark())
		}
	}
	if expect.SteadyAt != nil {
		field := "steadyAt"
		if failSet[field] {
			fmt.Printf("  %s steadyAt ≤%d\n", cmdutil.FailureMark(), *expect.SteadyAt)
		} else {
			fmt.Printf("  %s steadyAt ≤%d\n", cmdutil.SuccessMark(), *expect.SteadyAt)
		}
	}
	if expect.NoErrors {
		hasErr := false
		for f := range failSet {
			if len(f) > 6 && f[:6] == "cycles" {
				hasErr = true
				break
			}
		}
		if hasErr {
			fmt.Printf("  %s noErrors\n", cmdutil.FailureMark())
		} else {
			fmt.Printf("  %s noErrors\n", cmdutil.SuccessMark())
		}
	}
	for i, rule := range expect.Ops {
		field := fmt.Sprintf("ops[%d]", i)
		desc := rule.Verb + " " + rule.Resource
		if rule.Name != "" {
			desc += "/" + rule.Name
		}
		desc += fmt.Sprintf(" (cycle %d)", rule.Cycle)
		if failSet[field] {
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), desc)
		} else {
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), desc)
		}
	}
	for i, rule := range expect.Absent {
		field := fmt.Sprintf("absent[%d]", i)
		desc := "absent: " + rule.Verb + " " + rule.Resource
		if rule.Name != "" {
			desc += "/" + rule.Name
		}
		desc += fmt.Sprintf(" (cycle %d)", rule.Cycle)
		if failSet[field] {
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), desc)
		} else {
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), desc)
		}
	}
	fmt.Println()

	if len(errs) > 0 {
		fmt.Printf("  %s\n\n", cmdutil.Red("FAIL"))
	} else {
		fmt.Printf("  %s\n\n", cmdutil.Green("PASS"))
	}
}

// printSimulateAutoscaleSummary prints a one-line autoscale marker for each workload
// that declares autoscale:, so the policy is visible in simulate output.
func printSimulateAutoscaleSummary(entry types.CRDEntry) {
	type workload struct {
		name      string
		autoscale *types.WorkloadAutoscale
	}
	var workloads []workload
	for _, ht := range []*types.HookTemplates{entry.Box().EffectiveOnCreate(), entry.Box().EffectiveOnReconcile()} {
		if ht == nil {
			continue
		}
		for _, d := range ht.Deployments {
			if d.Autoscale != nil {
				workloads = append(workloads, workload{d.Name, d.Autoscale})
			}
		}
		for _, s := range ht.StatefulSets {
			if s.Autoscale != nil {
				workloads = append(workloads, workload{s.Name, s.Autoscale})
			}
		}
		for _, r := range ht.ReplicaSets {
			if r.Autoscale != nil {
				workloads = append(workloads, workload{r.Name, r.Autoscale})
			}
		}
	}
	for _, w := range workloads {
		a := w.autoscale
		min := int32(0)
		if a.Min != nil {
			min = *a.Min
		}
		cooldown := a.EffectiveCooldown().Duration.String()
		fmt.Printf("  %s %s  %s\n", cmdutil.Dim("autoscale:"), cmdutil.Gray(w.name),
			cmdutil.Gray(fmt.Sprintf("min=%d max=%d cooldown=%s", min, a.Max, cooldown)))
	}
}

// printCycleOps prints one cycle's ops, coalescing duplicate resource+name entries.
// If a resource is both created and updated in the same cycle (e.g. reconcile: true),
// only the create icon (+) is shown.
func printCycleOps(ops []simulate.Op) {
	type entry struct {
		resource  string
		name      string
		hasCreate bool
		hasDelete bool
	}
	seen := map[string]*entry{}
	var order []string
	for _, op := range ops {
		key := op.Resource + "/" + op.Name
		if _, ok := seen[key]; !ok {
			seen[key] = &entry{resource: op.Resource, name: op.Name}
			order = append(order, key)
		}
		switch op.Verb {
		case "create":
			seen[key].hasCreate = true
		case "delete":
			seen[key].hasDelete = true
		case "apply":
			// SSA ops display as ~ (changed), same as update/patch
		}
	}
	for _, key := range order {
		e := seen[key]
		var icon string
		switch {
		case e.hasCreate:
			icon = cmdutil.IconAdded()
		case e.hasDelete:
			icon = cmdutil.IconRemoved()
		default:
			icon = cmdutil.IconChanged()
		}
		fmt.Printf("    %s %s/%s\n", icon, e.resource, e.name)
	}
}

// opsKey returns a stable string key for a slice of ops, used to detect identical cycles.
func opsKey(ops []simulate.Op) string {
	var b strings.Builder
	for _, op := range ops {
		b.WriteString(op.Verb)
		b.WriteByte('/')
		b.WriteString(op.Resource)
		b.WriteByte('/')
		b.WriteString(op.Name)
		b.WriteByte('|')
	}
	return b.String()
}

func filterOps(ops []simulate.Op, verbs ...string) []simulate.Op {
	verbSet := make(map[string]bool)
	for _, v := range verbs {
		verbSet[v] = true
	}
	var result []simulate.Op
	for _, op := range ops {
		if verbSet[op.Verb] {
			result = append(result, op)
		}
	}
	return result
}
