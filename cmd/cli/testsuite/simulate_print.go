//go:build !runtime && !gateway

package testsuite

// Printing for inrun simulate: every function here renders a simulation, a
// run's cycles and assertions, or a comparison, and decides nothing. The
// simulate package runs, judges and compares; simulate.go and
// simulate_compare.go sequence that and call these.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
)

// underSpinner runs run under a spinner showing msg, with the reconciler's
// logs discarded. With verbose there is no spinner, so log lines do not
// break into it, and the logs show; errors always do.
func underSpinner(msg string, verbose bool, run func() (*simulate.Result, error)) (*simulate.Result, error) {
	if verbose {
		fmt.Println(msg)
		return run()
	}
	restore := logger.Quiet()
	defer restore()
	spin := cmdutil.StartSpinner(msg)
	res, err := run()
	if err != nil {
		spin.Failure()
	} else {
		spin.Stop()
	}
	return res, err
}

// printSimulateHeader prints what is simulated and where, then notes on
// what the backends cannot run as the cluster would.
func printSimulateHeader(opts simulate.RunOptions, simOpts CliSimulateOptions, backend string) {
	version := simOpts.K8sVersion
	if version == "" {
		version = simulate.DefaultEnvtestK8sVersion
	}
	switch backend {
	case "envtest":
		fmt.Printf("Simulating %s/%s   %s\n", opts.CRDName, opts.CR.GetName(), cmdutil.Gray("(envtest · kube-apiserver "+version+")"))
	case "compare":
		fmt.Printf("Simulating %s/%s   %s\n", opts.CRDName, opts.CR.GetName(), cmdutil.Gray("(compare · in-memory vs envtest · kube-apiserver "+version+")"))
	default:
		fmt.Printf("Simulating %s/%s\n", opts.CRDName, opts.CR.GetName())
	}

	crdEntry, _ := opts.Blueprint.CRDEntry(opts.CRDName)
	if crdEntry.Box().EffectiveOnReconcile() != nil && len(crdEntry.Box().EffectiveOnReconcile().External) > 0 {
		if opts.SkipExternal {
			fmt.Printf("  %s\n", cmdutil.Dim("note: external: calls stubbed — result fields will be empty"))
		} else {
			fmt.Printf("  %s\n", cmdutil.Dim("note: external: calls will hit the real network (pass --skip-external to stub)"))
		}
	}
	if len(crdEntry.Box().EffectiveCross()) > 0 && len(opts.Peers) == 0 {
		fmt.Printf("  %s\n", cmdutil.Dim("note: cross: peer CRs not provided — cross.* fields will be empty (add sibling CRs to the CR file)"))
	}
	printSimulateAutoscaleSummary(crdEntry)
	fmt.Println()
}

// printRun prints a run: with debugOps every recorded op, then the notes,
// each cycle's writes (identical cycles folded into one line), and whether
// it reached a steady state.
func printRun(result *simulate.Result, maxCycles int, debugOps bool) {
	if debugOps {
		fmt.Printf("  [debug-ops] %d total ops recorded across all cycles:\n", len(result.AllOps))
		for _, op := range result.AllOps {
			fmt.Printf("  [debug-ops]   cycle=%-2d  verb=%-8s  resource=%-20s  name=%s\n",
				op.Cycle, op.Verb, op.Resource, op.Name)
		}
		fmt.Println()
	}

	printNotes(result)
	if len(result.Notes) > 0 {
		fmt.Println()
	}

	var prevKey string
	var repeatStart int

	flush := func(upTo int) {
		if repeatStart > 0 && upTo > repeatStart {
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("(cycles %d–%d: identical)", repeatStart, upTo)))
			repeatStart = 0
		}
	}

	for _, cycle := range result.Cycles {
		meaningful := filterOps(cycle.Ops, types.WriteVerbs...)
		if len(meaningful) == 0 && cycle.Error == nil {
			continue
		}

		key := opsKey(meaningful)
		if key == prevKey && cycle.Error == nil {
			if repeatStart == 0 {
				repeatStart = cycle.Cycle
			}
			continue
		}

		flush(cycle.Cycle - 1)
		prevKey = key

		fmt.Printf("  Cycle %d:\n", cycle.Cycle)
		printCycleOps(meaningful)
		if cycle.Error != nil {
			fmt.Printf("    %s %v\n", cmdutil.FailureMark(), cycle.Error)
		}
	}

	if len(result.Cycles) > 0 {
		flush(result.Cycles[len(result.Cycles)-1].Cycle)
	}

	fmt.Println()
	printSteady(result, maxCycles)
	fmt.Println()
}

func printNotes(result *simulate.Result) {
	for _, note := range result.Notes {
		fmt.Printf("  %s\n", cmdutil.Dim("note: "+note))
	}
}

// printSteady prints whether the run reached a steady state, and how long
// it reconciled, the API server's start-up apart.
func printSteady(result *simulate.Result, maxCycles int) {
	if result.Steady {
		fmt.Printf("  %s Steady state at cycle %d in %s%s\n",
			cmdutil.SuccessMark(), result.SteadyAt, result.Elapsed.Round(time.Millisecond), startupNote(result))
	} else {
		fmt.Printf("  ~ Max cycles reached (%d) in %s%s\n",
			maxCycles, result.Elapsed.Round(time.Millisecond), startupNote(result))
	}
}

// startupNote names the API server's start-up time, kept apart from the
// time spent reconciling.
func startupNote(res *simulate.Result) string {
	if res.Startup == 0 {
		return ""
	}
	return " " + cmdutil.Gray(fmt.Sprintf("(+%s API server start)", res.Startup.Round(100*time.Millisecond)))
}

func printAssertions(errs simulate.AssertionResults, expect *types.SimulateExpect) {
	failSet := map[string]bool{}
	skipSet := map[string]bool{}

	for _, e := range errs {
		switch {
		case e.Skipped:
			skipSet[e.Field] = true
		case e.Message != "":
			failSet[e.Field] = true
		}
	}

	printStatus := func(field, desc string) {
		switch {
		case failSet[field]:
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), desc)
		case skipSet[field]:
			fmt.Printf("  %s %s (skipped)\n", cmdutil.Dim("–"), desc)
		default:
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), desc)
		}
	}

	if expect.Steady != nil {
		printStatus("steady", "steady")
	}
	if expect.SteadyAt != nil {
		printStatus("steadyAt", fmt.Sprintf("steadyAt ≤%d", *expect.SteadyAt))
	}
	if expect.NoErrors {
		printStatus("noErrors", "noErrors")
	}

	for i, rule := range expect.Ops {
		field := fmt.Sprintf("ops[%d]", i)

		var desc string
		if rule.Verb == "" && rule.Resource == "" {
			if rule.Name != "" {
				desc = rule.Name
			} else {
				desc = "assert"
			}
		} else {
			desc = rule.Verb + " " + ruleTarget(rule)
		}
		desc += fmt.Sprintf(" (cycle %d)", rule.Cycle)

		printStatus(field, desc)
	}

	for i, rule := range expect.Absent {
		field := fmt.Sprintf("absent[%d]", i)

		desc := "absent: " + rule.Verb + " " + ruleTarget(rule)
		desc += fmt.Sprintf(" (cycle %d)", rule.Cycle)

		printStatus(field, desc)
	}

	fmt.Println()

	if len(failSet) > 0 {
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

// printAssertionSummary prints one run's assertions on a line, with each
// failure under it; without expectations, only a cycle-error failure.
func printAssertionSummary(res *simulate.Result, errs simulate.AssertionResults, expect *types.SimulateExpect, evalErr error) {
	if expect == nil {
		if evalErr != nil {
			fmt.Printf("  %s %v\n", cmdutil.FailureMark(), evalErr)
		}
		fmt.Println()
		return
	}
	a := res.Assertions
	mark := cmdutil.SuccessMark()
	if a.Failed > 0 {
		mark = cmdutil.FailureMark()
	}
	fmt.Printf("  %s %d/%d assertion(s) passed\n", mark, a.Passed, a.Total)
	for _, e := range errs {
		if !e.Skipped {
			fmt.Printf("      %s\n", cmdutil.Red(e.Error()))
		}
	}
	fmt.Println()
}

// printComparison prints the write and object comparison. Fields only
// envtest holds that no apply sent (API-server defaults) are counted per
// object, and listed with verbose.
func printComparison(cmp *simulate.Comparison, verbose bool) {
	if !cmp.Complete() {
		return
	}

	fmt.Printf("  Comparing writes...\n")
	if cmp.Writes.Matched > 0 {
		fmt.Printf("  %s %d write(s) match\n", cmdutil.SuccessMark(), cmp.Writes.Matched)
	}
	for _, d := range cmp.Writes.Differences {
		fmt.Printf("  %s %s %s\n", cmdutil.FailureMark(), d.Verb, d.Ref())
		fmt.Printf("      in-memory: %s\n", writeCount(d.Memory, d.MemoryCycles))
		fmt.Printf("      envtest:   %s\n", writeCount(d.Envtest, d.EnvtestCycles))
	}
	fmt.Println()

	fmt.Printf("  Comparing final objects...\n")
	for _, o := range cmp.Objects {
		switch {
		case !o.InEnvtest:
			fmt.Printf("  %s %s\n      only in-memory — envtest does not hold it\n", cmdutil.FailureMark(), o.Ref())
		case !o.InMemory:
			fmt.Printf("  %s %s\n      only in envtest — the in-memory cluster does not hold it\n", cmdutil.FailureMark(), o.Ref())
		case len(o.Differences) > 0:
			fmt.Printf("  %s %s%s\n", cmdutil.FailureMark(), o.Ref(), fieldNote(o))
			for _, d := range o.Differences {
				fmt.Printf("      %s\n", d.Path)
				fmt.Printf("        in-memory: %s\n", fieldValue(d.Memory))
				fmt.Printf("        envtest:   %s\n", fieldValue(d.Envtest))
			}
		default:
			fmt.Printf("  %s %s%s\n", cmdutil.SuccessMark(), o.Ref(), fieldNote(o))
		}
		if verbose {
			for _, d := range o.Defaulted {
				fmt.Printf("      %s %s = %s\n", cmdutil.Dim("defaulted"), d.Path, fieldValue(d.Envtest))
			}
			for _, d := range o.Varies {
				fmt.Printf("      %s %s: %s / %s\n", cmdutil.Dim("varies"), d.Path, fieldValue(d.Memory), fieldValue(d.Envtest))
			}
		}
	}
	fmt.Println()
}

// fieldNote counts the fields that differ for a known reason: defaulted by
// the API server, or random by design.
func fieldNote(o simulate.ObjectComparison) string {
	var parts []string
	if n := len(o.Defaulted); n > 0 {
		parts = append(parts, fmt.Sprintf("%d field(s) defaulted by the API server", n))
	}
	if n := len(o.Varies); n > 0 {
		parts = append(parts, fmt.Sprintf("%d field(s) vary: random by design", n))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + cmdutil.Gray("("+strings.Join(parts, "; ")+")")
}

func writeCount(n int, cycles []int) string {
	if n == 0 {
		return "not written"
	}
	parts := make([]string, len(cycles))
	for i, c := range cycles {
		parts[i] = fmt.Sprint(c)
	}
	return fmt.Sprintf("%d× (cycle %s)", n, strings.Join(parts, ", "))
}

// fieldValue renders a field's value on one line, "absent" when the run
// does not hold it.
func fieldValue(v any) string {
	if v == nil {
		return "absent"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(raw)
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}

// ruleTarget names what an op rule matches: resource, /name, and
// "in namespace" when the rule names one.
func ruleTarget(rule types.SimulateOpRule) string {
	t := rule.Resource
	if rule.Name != "" {
		t += "/" + rule.Name
	}
	if rule.Namespace != "" {
		t += " in " + rule.Namespace
	}
	return t
}
