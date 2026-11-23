package e2e

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/jig"
)

// New loads an E2E spec from a YAML file and constructs a Runner.
func New(e2eFile string, opts Options) (*Runner, error) {
	data, err := readLocal(e2eFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", e2eFile, err)
	}

	var e2e types.E2E
	if err := strictUnmarshal(data, &e2e); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", e2eFile, err)
	}
	if e2e.Kind != "E2E" {
		return nil, fmt.Errorf("%s: expected kind E2E, got %q", e2eFile, e2e.Kind)
	}

	e2eDir, err := filepath.Abs(filepath.Dir(e2eFile))
	if err != nil {
		return nil, fmt.Errorf("resolving e2e directory: %w", err)
	}
	allValueFiles := make([]string, 0, len(e2e.Spec.ValuesFiles)+len(opts.ValueFiles))
	for _, f := range e2e.Spec.ValuesFiles {
		if !filepath.IsAbs(f) {
			f = filepath.Join(e2eDir, f)
		}
		allValueFiles = append(allValueFiles, f)
	}
	allValueFiles = append(allValueFiles, opts.ValueFiles...)

	noRuntime := opts.NoRuntime || !e2e.Spec.RuntimeEnabled()

	// pass inheritance
	e2e.Spec.InheritEnv(opts.Env)

	r := &Runner{
		e2e:          e2e,
		e2eDir:       e2eDir,
		e2eFile:      e2eFile,
		devServer:    opts.DevServer,
		inrunVersion: opts.InrunVersion,
		inrunChart:   opts.InrunChart,
		valueFiles:   allValueFiles,
		helmArgs:     opts.HelmArgs,
		noRuntime:    noRuntime,
		opts:         opts,
	}

	if err := r.resolveSource(); err != nil {
		return nil, err
	}
	if err := r.validateImports(); err != nil {
		return nil, err
	}

	// Merge blueprint functions into the E2E spec so expectations can use them.
	// E2E-level definitions win on name collision — they are the test's overrides.
	if r.blueprintFile != "" {
		bp, err := blueprint.ParseFile(r.blueprintFile)
		if err != nil {
			return nil, fmt.Errorf("parsing blueprint for functions: %w", err)
		}
		merged, err := r.e2e.Spec.Functions.Merge(bp.UserFunctions(), "blueprint")
		if err != nil {
			return nil, fmt.Errorf("merging blueprint functions: %w", err)
		}
		r.e2e.Spec.Functions = merged
	}

	return r, nil
}

// RunFromFile runs the test at path: an Inrun e2e file (kind: E2E) through
// ToJig, any other file as a plain jig test.
func RunFromFile(ctx context.Context, path string, opts Options) (*jig.Result, error) {
	if IsE2EDoc(path) {
		r, err := New(path, opts)
		if err != nil {
			return nil, err
		}
		return r.Run(ctx)
	}
	s, err := jig.FromFile(path)
	if err != nil {
		return nil, err
	}
	r := &Runner{e2eFile: path, opts: opts}
	return r.runJig(ctx, s, nil)
}

// Run maps the test to jig (ToJig) and runs it with jig.Run.
func (r *Runner) Run(ctx context.Context) (*jig.Result, error) {
	s, cleanup, err := r.ToJig(ctx)
	if err != nil {
		cleanup()
		return nil, err
	}
	return r.runJig(ctx, s, cleanup)
}

// runJig runs t with jig.Run. Its imports are mapped with ToJig when they
// are Inrun e2e files and read as plain jig tests when not. cleanup, and
// the imports' cleanups, run afterwards: they remove the files the mapping
// wrote.
func (r *Runner) runJig(ctx context.Context, d *jig.Declaration, cleanup func()) (*jig.Result, error) {
	cleanups := []func(){}
	if cleanup != nil {
		cleanups = append(cleanups, cleanup)
	}
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()
	o := r.opts
	specPath := r.e2eFile
	if o.PushTriggered {
		specPath = r.specPathRel()
	}
	return jig.Run(ctx, d, jig.Options{
		Context:            o.Context,
		UseCurrent:         o.UseCurrentCtx,
		KeepCluster:        o.KeepCluster,
		Keep:               o.Keep,
		Workers:            o.Workers,
		KindVersion:        o.KindVersion,
		KindBinDir:         inruncluster.BinDir(),
		DefaultClusterName: types.DefaultE2EClusterName,
		Images:             o.Images,
		Functions:          o.Functions,
		Env:                o.Env,
		MaxFailures:        o.MaxFailures,
		DryRun:             o.DryRun,
		ReportFile:         o.ReportFile,
		Caller:             "E2E",
		Command:            o.Command,
		SpecPath:           specPath,
		FieldManager:       "inrun-e2e",
		Load: func(path string) (*jig.Declaration, error) {
			if !IsE2EDoc(path) {
				return jig.FromFile(path)
			}
			sub, err := New(path, r.importOptions())
			if err != nil {
				return nil, err
			}
			s, cleanup, err := sub.ToJig(ctx)
			cleanups = append(cleanups, cleanup)
			return s, err
		},
	})
}

// importOptions are what an imported Inrun e2e file runs with: the Inrun
// install and dev server as this run's. jig passes the env and the cluster.
func (r *Runner) importOptions() Options {
	return Options{
		DevServer:    r.opts.DevServer,
		InrunVersion: r.opts.InrunVersion,
		InrunChart:   r.opts.InrunChart,
		ValueFiles:   r.opts.ValueFiles,
		HelmArgs:     r.opts.HelmArgs,
		NoRuntime:    r.opts.NoRuntime,
		Functions:    r.opts.Functions,
	}
}
