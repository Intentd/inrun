package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/jig"
)

// Map maps the Inrun e2e test to a jig test, offline, the way
// declarationFor maps a migration for KOME: Inrun's own parts become
// ordinary steps. The files Inrun writes at run time (the bundle, the dev
// server manifest) are named and marked Generated, so jig.Validate doesn't
// look for them, and nothing asks a cluster anything. inrun validate checks
// and prints what Map returns; ToJig makes it runnable.
//
// The steps, in order: the CRDs; the bundle; the e2e prepare's manifests and
// charts; the dev server; Inrun, installed or upgraded; the CRs (cr); their
// deletion (cr-deleted). A server dry run installs nothing, so it gets only
// the CR steps. Expectations after setup-complete run after the step before
// cr, cr-applied after cr, cr-deleted after cr-deleted.
func (r *Runner) Map() (*jig.Test, error) {
	e := r.e2e.Spec
	expects, err := e.Expect.ExpandIncludes(r.e2eDir)
	if err != nil {
		return nil, err
	}
	t := &jig.Test{
		Name:        r.e2e.Metadata.Name,
		Description: r.e2e.Metadata.Description,
		Env:         e.Env,
		Functions:   jigFunctions(e.Functions),
		Environment: e.Environment,
		OnFailure:   e.OnFailure,
		Finally:     e.Finally,
		Imports:     r.e2e.Imports,
		Dir:         r.e2eDir,
		Data:        loadCRData(&r.e2e.Spec, r.e2eDir),
	}

	install := e.Environment.Provider() != types.ProviderNone && !r.opts.DryRun
	if install {
		crds, err := r.crdPaths()
		if err != nil {
			return nil, err
		}
		if len(crds) > 0 {
			t.Prepare = append(t.Prepare, jig.Step{Name: stepCRDs, Apply: entries(crds)})
		}
		if r.blueprintFile != "" {
			// Kept for the imports after this one: it holds inrun-system,
			// and each import applies its own over it.
			t.Prepare = append(t.Prepare, jig.Step{Name: stepBundle, Keep: true, Apply: jig.ApplyEntries{
				{Path: "the bundle of " + r.rel(r.blueprintFile), Generated: true},
			}})
		}
		t.Prepare = append(t.Prepare, prepareSteps(e.Prepare)...)
		// After the bundle, which creates inrun-system: an aggregator, or a
		// test without a blueprint, has no namespace to put it in.
		if r.devServer && r.blueprintFile != "" {
			t.Prepare = append(t.Prepare, jig.Step{
				Name:  stepDevServer,
				Apply: jig.ApplyEntries{{Path: "the dev server manifest", Generated: true}},
				Wait:  jig.Waits{{Kind: "Deployment", Name: devServerName, Namespace: devServerNamespace, Ready: true, Timeout: "120s"}},
			})
		}
		if r.blueprintFile != "" {
			step, err := r.inrunStep()
			if err != nil {
				return nil, err
			}
			t.Prepare = append(t.Prepare, step)
		}
	}

	setup := jig.AfterEnvironment
	if n := len(t.Prepare); n > 0 {
		setup = jig.After(t.Prepare[n-1].Name)
	}
	crApplied, crDeleted := jig.AfterPrepare, jig.AfterPrepare
	cluster := e.Environment.Provider() != types.ProviderNone
	if cluster && len(r.crFiles) > 0 && !r.noRuntime {
		t.Prepare = append(t.Prepare, jig.Step{Name: stepCR, Apply: entries(r.crFiles)})
		crApplied = stepCR
		for _, x := range expects {
			if x.After == types.AfterCRDeleted {
				t.Prepare = append(t.Prepare, jig.Step{Name: stepCRDeleted, Delete: r.crFiles})
				crDeleted = stepCRDeleted
				break
			}
		}
	}
	for _, x := range expects {
		switch x.After {
		case "", types.AfterSetupComplete:
			x.After = setup
		case types.AfterCRApplied:
			x.After = crApplied
		case types.AfterCRDeleted:
			x.After = crDeleted
		}
		t.Expect = append(t.Expect, x)
	}
	return t, nil
}

// The steps Map emits.
const (
	stepCRDs      = "crds"
	stepBundle    = "bundle"
	stepDevServer = "dev-server"
	stepInrun     = "inrun"
	stepCR        = "cr"
	stepCRDeleted = "cr-deleted"
)

// ToJig is Map made runnable: the OCI imports pulled, the bundle generated
// and the dev server manifest written in place of their names, and Inrun
// restarted after its upgrade when it is already running (an import on its
// parent's cluster), so it reads the new bundle. The cleanup removes the
// files it wrote.
func (r *Runner) ToJig(ctx context.Context) (*jig.Test, func(), error) {
	var files []string
	cleanup := func() {
		for _, f := range files {
			os.Remove(f)
		}
	}
	t, err := r.Map()
	if err != nil {
		return nil, cleanup, err
	}
	for i := range t.Prepare {
		st := &t.Prepare[i]
		switch st.Name {
		case stepBundle:
			if err := r.pullOCIImports(ctx); err != nil {
				return nil, cleanup, fmt.Errorf("pulling OCI imports: %w", err)
			}
			bundle, err := r.generateBundle(ctx)
			if err != nil {
				return nil, cleanup, fmt.Errorf("generate bundle: %w", err)
			}
			files = append(files, bundle)
			st.Apply = entries([]string{bundle})
		case stepDevServer:
			manifest, err := writeDevServerManifest()
			if err != nil {
				return nil, cleanup, err
			}
			files = append(files, manifest)
			st.Apply = entries([]string{manifest})
		case stepInrun:
			st.Restart = r.inrunRestarts(st)
		}
	}
	if problems := jig.Validate(t); len(problems) > 0 {
		return nil, cleanup, fmt.Errorf("mapping %s to jig: %w", r.e2eFile, errorsJoin(problems))
	}
	return t, cleanup, nil
}

// inrunRestarts restarts the runtime and the gateway the inrun step waits
// for, when they already run: an upgrade alone leaves them on the old
// bundle.
func (r *Runner) inrunRestarts(st *jig.Step) jig.Restarts {
	var restart jig.Restarts
	for _, w := range st.Helm.Wait {
		installed := false
		switch w.Name {
		case inruncluster.InrunRuntime:
			installed = inruncluster.RuntimeInstalled()
		case inruncluster.InrunGateway:
			installed = inruncluster.GatewayInstalled()
		}
		if installed {
			restart = append(restart, jig.Restart{Kind: w.Kind, Name: w.Name, Namespace: w.Namespace})
		}
	}
	return restart
}

// rel names path relative to the test's directory, for output.
func (r *Runner) rel(path string) string {
	if p, err := filepath.Rel(r.e2eDir, path); err == nil {
		return p
	}
	return path
}

// inrunStep installs or upgrades Inrun with the run's chart, version, values
// and --set arguments: the console off, the gateway on when the Blueprint
// has one, the runtime off with --no-runtime, and a wait on each that runs.
// ToJig adds the restart when Inrun already runs.
func (r *Runner) inrunStep() (jig.Step, error) {
	values, err := setValues(r.helmArgs)
	if err != nil {
		return jig.Step{}, err
	}
	values["console.enabled"] = false
	gateway, err := resolveGatewayEnabled(r.blueprintFile)
	if err != nil {
		return jig.Step{}, err
	}
	if gateway {
		values["gateway.enabled"] = true
	}
	if r.noRuntime {
		values["runtime.enabled"] = false
	}
	h := jig.HelmInstall{
		Chart:           r.inrunChart,
		Release:         inruncluster.Inrun,
		Namespace:       inruncluster.InrunNamespace,
		Version:         r.inrunVersion,
		ValueFiles:      r.valueFiles,
		Values:          values,
		CreateNamespace: true,
	}
	if h.Chart == "" {
		h.Repo, h.Chart = inruncluster.InrunChartRepo, inruncluster.InrunChartName
	}
	if !r.noRuntime {
		h.Wait = append(h.Wait, jig.Wait{Kind: "Deployment", Name: inruncluster.InrunRuntime, Namespace: inruncluster.InrunNamespace, Ready: true})
	}
	if gateway {
		h.Wait = append(h.Wait, jig.Wait{Kind: "Deployment", Name: inruncluster.InrunGateway, Namespace: inruncluster.InrunNamespace, Ready: true})
	}
	// Kept for the imports after this one, which upgrade it with their own
	// bundle; the parent uninstalls it after the last.
	return jig.Step{Name: stepInrun, Helm: &h, Keep: true}, nil
}

// prepareSteps maps the e2e prepare: its manifests as one step, each chart as
// one, its waits on the last of them.
func prepareSteps(p *types.Prepare) jig.Steps {
	if p == nil {
		return nil
	}
	var steps jig.Steps
	if len(p.Apply) > 0 {
		steps = append(steps, jig.Step{Name: "prepare-apply", Apply: p.Apply})
	}
	for i, h := range p.Helm {
		h := h
		steps = append(steps, jig.Step{Name: fmt.Sprintf("prepare-helm-%d", i+1), Helm: &h})
	}
	if len(p.Wait) > 0 {
		if len(steps) == 0 {
			steps = append(steps, jig.Step{Name: "prepare-wait", Commands: jig.Commands{{Run: "true"}}})
		}
		last := &steps[len(steps)-1]
		last.Wait = append(last.Wait, p.Wait...)
	}
	return steps
}

// crdPaths are the CRDs to apply: the spec's, else the Blueprint's crdFiles,
// a declared transitional CRD in its crdFile's place. Absolute, or a URL.
func (r *Runner) crdPaths() ([]string, error) {
	if paths := r.e2e.Spec.AllCRDPaths(); len(paths) > 0 {
		out := make([]string, len(paths))
		for i, p := range paths {
			out[i] = r.abs(p)
		}
		return out, nil
	}
	if r.blueprintFile == "" {
		return nil, nil
	}
	var raw struct {
		Spec struct {
			CRDs map[string]struct {
				CRDFile string `yaml:"crdFile"`
				Migrate struct {
					Transitional struct {
						CRDFile string `yaml:"crdFile"`
					} `yaml:"transitional"`
				} `yaml:"migrate"`
			} `yaml:"crds"`
		} `yaml:"spec"`
	}
	data, err := readLocal(r.blueprintFile)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	dir := filepath.Dir(r.blueprintFile)
	var out []string
	for _, entry := range raw.Spec.CRDs {
		file := entry.CRDFile
		if t := entry.Migrate.Transitional.CRDFile; t != "" {
			file = t
		}
		switch {
		case file == "":
		case filepath.IsAbs(file), strings.Contains(file, "://"):
			out = append(out, file)
		default:
			out = append(out, filepath.Join(dir, file))
		}
	}
	return out, nil
}

// setValues reads --set k=v pairs into values.
func setValues(args []string) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		var kv string
		switch {
		case a == "--set" && i+1 < len(args):
			i++
			kv = args[i]
		case strings.HasPrefix(a, "--set="):
			kv = strings.TrimPrefix(a, "--set=")
		default:
			return nil, fmt.Errorf("helm argument %q: only --set k=v is supported", a)
		}
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("helm --set %q: want key=value", kv)
		}
		values[k] = v
	}
	return values, nil
}

func entries(paths []string) jig.ApplyEntries {
	out := make(jig.ApplyEntries, len(paths))
	for i, p := range paths {
		out[i] = jig.ApplyEntry{Path: p}
	}
	return out
}

// jigFunctions maps Inrun's functions to jig's.
func jigFunctions(r types.KELFunctionRegistry) jig.Functions {
	var f jig.Functions
	for _, d := range r.Definitions {
		f.Definitions = append(f.Definitions, jig.Function{
			Name:        d.Name,
			Description: d.Description,
			Expression:  d.Expression,
		})
	}
	return f
}

func errorsJoin(problems []error) error {
	msg := make([]string, len(problems))
	for i, p := range problems {
		msg[i] = "  - " + p.Error()
	}
	return fmt.Errorf("\n%s", strings.Join(msg, "\n"))
}
