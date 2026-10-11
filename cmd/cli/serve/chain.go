//go:build !runtime && !gateway

package serve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/gate"
	"github.com/intentd/inrun/cmd/cli/testsuite"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// planChain runs stages 1-6 of the play chain against an already-resolved
// intent map, reporting each stage through n: stageNarrator for serve play,
// quietNarrator for simulate. The steps are the same either way.
func planChain(c *catalog.Catalog, raw map[string]interface{}, tokenName string, op types.ServeOperation, n narrator) (*unstructured.Unstructured, *types.CRDEntry, string, error) {
	target, _ := raw["target"].(string)
	if strings.TrimSpace(target) == "" {
		return nil, nil, "", fmt.Errorf(`"target" must be set in the intent`)
	}

	// Stage 1: Target resolution
	n.stage(1, "Target resolution")
	resolution := c.LookupByTargetOrAlias(target)
	if resolution == nil {
		err := fmt.Errorf("unknown target %q\n  available: %s", target, strings.Join(c.AvailableTargets(), ", "))
		n.fail(err.Error())
		return nil, nil, "", err
	}
	crd, alias := resolution.CRD, resolution.Alias
	n.ok(fmt.Sprintf("kind=%s  target=%s  alias=%s", cmdutil.Bold(crd.Kind()), cmdutil.Green(crd.ServeTarget()), aliasOrNone(alias)))

	// Stage 2: Token check
	n.stage(2, "Token check")
	if err := checkToken(c, crd, alias, tokenName, op, ""); err != nil {
		n.fail(err.Error())
		return nil, nil, "", err
	}
	n.ok(fmt.Sprintf("token %s can %s on %s", cmdutil.Bold(tokenName), op, cmdutil.Bold(crd.Kind())))

	// Stage 3: CR construction
	n.stage(3, "CR construction")
	notes := c.Notes
	obj, err := intent.Build(raw, crd, notes)
	if err != nil {
		n.fail(err.Error())
		return nil, nil, "", err
	}
	if obj.GetName() == "" {
		err := fmt.Errorf("name is required — declare serve.name on the CRD or include \"name\" in the intent")
		n.fail(err.Error())
		return nil, nil, "", err
	}
	n.ok(fmt.Sprintf("name=%s  namespace=%s", cmdutil.Bold(obj.GetName()), cmdutil.Bold(obj.GetNamespace())))

	// Stage 4: Provenance — stamp first, then report the CR so annotations are visible
	n.stage(4, "Provenance annotations")
	api.InjectProvenanceAnnotations(obj, crd.ServeTarget(), alias)
	api.InjectServeIntentAnnotation(obj, raw)
	ann := obj.GetAnnotations()
	keys := make([]string, 0, len(ann))
	for k := range ann {
		if strings.HasPrefix(k, "inrun.") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		n.detail(fmt.Sprintf("%s: %s", cmdutil.Gray(k), cmdutil.Cyan(ann[k])))
	}
	n.ok("provenance stamped — full CR:")
	n.json(obj.Object)

	// Stage 5: Admission validation — same rules the webhook enforces at SSA time.
	// unique: operator is skipped (no live cluster); external: calls are skipped.
	n.stage(5, "Admission validation")
	{
		resolver := template.NewResolverFromMap(obj.Object).WithUserNotes(c.Notes).WithRequest(raw)
		eval := resolver.TemplateEvaluator()
		r := gate.EvalAdmissionValidation(obj.Object, crd, resolver, eval)
		for _, v := range r.Violations {
			if v.Deny {
				n.detail(fmt.Sprintf("%s %s: %s", cmdutil.Red("deny"), cmdutil.Gray(v.Field), cmdutil.Red(v.Message)))
			} else {
				n.detail(fmt.Sprintf("%s %s: %s", cmdutil.Yellow("warn"), cmdutil.Gray(v.Field), cmdutil.Yellow(v.Message)))
			}
		}
		if denied := r.Denied(); denied > 0 {
			n.fail(fmt.Sprintf("%d %s — this CR would be rejected at admission", denied, plural(denied, "denial", "denials")))
			return nil, nil, "", fmt.Errorf("admission denied: %d rule %s", denied, plural(denied, "violation", "violations"))
		}
		switch warned := r.Warned(); {
		case warned > 0:
			n.ok(fmt.Sprintf("passed (%d %s)", warned, plural(warned, "warning", "warnings")))
		case r.Total > 0:
			n.ok("passed — no violations")
		default:
			n.ok("no validation rules declared")
		}

		m := gate.EvalAdmissionMutation(obj.Object, crd, resolver, eval)
		for _, p := range m.Previews {
			fromStr := cmdutil.Gray("(absent)")
			if p.Found {
				fromStr = cmdutil.Gray(p.From)
			}
			n.detail(fmt.Sprintf("%s %s: %s → %s  %s", cmdutil.Cyan("mut"), cmdutil.Gray(p.Field), fromStr, cmdutil.Cyan(fmt.Sprintf("%v", p.To)), cmdutil.Gray(p.MutType)))
		}
		if len(m.Previews) > 0 {
			n.ok(fmt.Sprintf("%d %s would be applied by the webhook", len(m.Previews), plural(len(m.Previews), "mutation", "mutations")))
		}
	}

	// Stage 6: Response payload
	n.stage(6, "Response payload")
	if payload := api.EvaluatePayload(obj.Object, crd, alias, notes); payload == nil {
		n.detail(cmdutil.Gray("no serve.config.response declared — default CR response"))
	} else {
		n.json(payload)
	}
	n.ok("payload evaluated")

	return obj, crd, alias, nil
}

// playRunSimulate writes the built CR to a temp file and hands it to simulate.
// simulateConfig is the path to a simulate.yaml; empty means op-print mode with
// the same catalog play used. Shared by "inrun serve play --simulate" and
// "inrun webhook play --simulate" via playSimulate below.
func playRunSimulate(ctx context.Context, catalogFile string, obj *unstructured.Unstructured, simulateConfig string, simOpts testsuite.CliSimulateOptions) error {
	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return fmt.Errorf("marshalling CR for simulate: %w", err)
	}
	tmp, err := os.CreateTemp("", "play-cr-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp CR file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp CR file: %w", err)
	}
	tmp.Close()

	fmt.Printf("%s  handoff to simulate\n", cmdutil.Cyan("─────────────────────────────────────────────────────────"))
	fmt.Println()

	if simulateConfig != "" {
		return runSimulateWithCR(ctx, simulateConfig, tmp.Name(), simOpts)
	}
	simOpts.MaxCycles = 10
	simOpts.SkipExternal = true
	return testsuite.RunSimulate(ctx, catalogFile, tmp.Name(), simOpts)
}

// runSimulateWithCR runs simulate using the spec file for catalog/cycles/expect
// but substitutes crFile (the play-built CR) instead of spec.cr.
// The expect: block is fully evaluated — this is assert mode, not op-print mode.
func runSimulateWithCR(ctx context.Context, specPath, crFile string, simOpts testsuite.CliSimulateOptions) error {
	if abs, err := filepath.Abs(specPath); err == nil {
		specPath = abs
	}
	data, err := cmdutil.ReadLocal(specPath)
	if err != nil {
		return fmt.Errorf("reading simulate config %q: %w", specPath, err)
	}
	var doc types.Simulate
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing simulate config %q: %w", specPath, err)
	}
	if doc.Spec == nil {
		return fmt.Errorf("simulate config %q: missing spec", specPath)
	}

	catalogPath := cmdutil.JoinPath(specPath, doc.Spec.Catalog)
	cycles := doc.Spec.Cycles
	if cycles <= 0 {
		cycles = 10
	}
	opts := simulate.RunOptions{SkipExternal: doc.Spec.SkipExternal}

	m := merger.New(catalogPath)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging Catalog: %w", err)
	}
	// --envtest installs the spec's CRDs, else the ones the Catalog points at.
	var crdPaths []string
	for _, p := range doc.Spec.AllCRDPaths() {
		crdPaths = append(crdPaths, cmdutil.JoinPath(specPath, p))
	}
	if len(crdPaths) == 0 {
		crdPaths = testsuite.CatalogCRDPaths(m)
	}
	cat, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		return fmt.Errorf("parsing Catalog: %w", err)
	}

	crData, err := cmdutil.ReadLocal(crFile)
	if err != nil {
		return fmt.Errorf("reading CR: %w", err)
	}
	crs := cmdutil.ParseMultiDocCRs(crData)
	if len(crs) == 0 {
		return fmt.Errorf("no valid CR documents in %s", crFile)
	}

	var failed []string
	for _, name := range cat.CRDNames() {
		crdEntry, ok := cat.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			if len(cat.CRDNames()) > 1 {
				fmt.Printf("  %s no CR found for %s — skipped\n\n", cmdutil.Dim("note:"), crdEntry.APITypes.Kind)
				continue
			}
			return fmt.Errorf("play: no CR found for CRD %q (kind: %s)", name, crdEntry.APITypes.Kind)
		}
		crdOpts := opts
		crdOpts.Peers = in.Peers
		crdOpts.ExistingInstances = in.Existing
		expect := simulate.ExpectForCRD(doc.Spec.Expect, name)
		if err := testsuite.SimulateOne(ctx, cat, name, in.CR, cycles, crdOpts, simOpts, crdPaths, expect); err != nil {
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("simulate failed for: %s", strings.Join(failed, ", "))
	}
	return nil
}

// RunIntentPlay builds a Catalog from catalogPath, runs the play chain
// against intentFile without printing stage output, and returns the resolved
// target name on success.
func RunIntentPlay(catalogPath, intentFile string) (string, error) {
	m := merger.New(catalogPath)
	if err := m.Merge(); err != nil {
		return "", fmt.Errorf("merging catalog: %w", err)
	}
	c, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		return "", fmt.Errorf("building catalog: %w", err)
	}

	raw, err := readIntentFile(intentFile)
	if err != nil {
		return "", fmt.Errorf("reading intent file: %w", err)
	}

	target, _ := raw["target"].(string)
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf(`intent file must declare a "target"`)
	}

	resolution := c.LookupByTargetOrAlias(target)
	if resolution == nil {
		return target, fmt.Errorf("unknown target %q — available: %s", target, strings.Join(c.AvailableTargets(), ", "))
	}
	crd, alias := resolution.CRD, resolution.Alias

	if tokenName, _ := raw["token"].(string); tokenName != "" {
		allowed, denyReason := crd.TokenAllowedFor(alias, tokenName, string(types.ServeOpCreate), "", types.ServeClassResources)
		if !allowed {
			return target, fmt.Errorf("token %q denied: %s", tokenName, denyReason.Message(tokenName, string(types.ServeOpCreate), crd.Kind(), ""))
		}
	} else {
		return target, fmt.Errorf("intent file must declare a 'token' — token: <name>")
	}

	obj, err := intent.Build(raw, crd, c.Notes)
	if err != nil {
		return target, fmt.Errorf("CR construction: %w", err)
	}

	resolver := template.NewResolverFromMap(obj.Object).WithUserNotes(c.Notes).WithRequest(raw)
	eval := resolver.TemplateEvaluator()
	r := gate.EvalAdmissionValidation(obj.Object, crd, resolver, eval)
	denied := r.Denied()
	violationTxt := "violations"
	if denied > 0 {
		if denied == 1 {
			violationTxt = "violation"
		}
		return target, fmt.Errorf("admission denied: %d rule %s", denied, violationTxt)
	}

	return target, nil
}

// A Simulate spec with spec.intent builds its CR through this same chain.
func init() {
	testsuite.PlayIntent = func(req testsuite.PlayRequest) (*unstructured.Unstructured, error) {
		raw, err := readIntentFile(req.IntentFile)
		if err != nil {
			return nil, fmt.Errorf("reading intent file %q: %w", req.IntentFile, err)
		}
		target, _ := raw["target"].(string)
		printPlayHeaderForSimulate(req.CatalogFile, req.IntentFile, target, req.Token, req.Cycles)

		var n narrator = quietNarrator{}
		if req.Verbose {
			n = stageNarrator{}
		}
		obj, _, _, err := planChain(req.Catalog, raw, req.Token, req.Op, n)
		if err == nil && !req.Verbose {
			printPlayed(req.IntentFile, obj)
		}
		return obj, err
	}
}
