package e2e

import (
	"context"
	"path/filepath"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/types"
)

func (r *Runner) runExpectations(
	ctx context.Context,
	name string,
	start time.Time,
	expects []types.E2EExpectation,
	planned int,
) (*expectationResult, error) {
	cases, failures, stopped, stopReason, err := expect.Run(ctx, expects, expect.Options{
		CS:          r.cs,
		Cfg:         r.cfg,
		Spec:        &r.e2e.Spec,
		CRFiles:     r.crFiles,
		WorkDir:     r.e2eDir,
		NoRuntime:   r.noRuntime,
		MaxFailures: r.maxFailures,
	})
	if err != nil {
		return nil, err
	}

	specPath := r.e2eFile
	if r.pushTriggered {
		specPath = r.specPathRel()
	}

	result := &expect.Result{
		Name:       name,
		Cases:      cases,
		Elapsed:    time.Since(start),
		Command:    r.command,
		Planned:    planned,
		SpecPath:   specPath,
		ClusterCtx: r.clusterCtx,
		Cluster:    r.clusterName(),
	}

	if stopped != nil {
		result.StoppedAfter = failures
		result.StopReason = stopReason
	}

	return &expectationResult{result: result, failures: failures, stopped: stopped}, nil
}

// specPathRel names the spec file relative to the pattern root, the way
// Files: lists it: "test/e2e.yaml".
func (r *Runner) specPathRel() string {
	if r.blueprintFile == "" {
		return filepath.Base(r.e2eFile)
	}
	base := filepath.Dir(r.blueprintFile)
	if rel, err := filepath.Rel(base, r.e2eFile); err == nil {
		return rel
	}
	return filepath.Base(r.e2eFile)
}
