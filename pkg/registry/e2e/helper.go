package e2e

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/kel/core"
	"github.com/intentd/tools/fileio"
	"github.com/intentd/tools/structured"
	"github.com/intentd/tools/term"
)

var (
	readLocal         = fileio.ReadLocal
	toAbs             = fileio.ToAbs
	strictUnmarshal   = structured.StrictUnmarshal
	startSpinner      = term.StartSpinner
	successMark       = term.SuccessMark
	failureMark       = term.FailureMark
	parseTimeDuration = core.ParseTimeDuration
)

// resolveSource resolves the blueprint and CR file paths from the spec.
func (r *Runner) resolveSource() error {
	spec := r.e2e.Spec

	switch {
	case spec.Init != nil:
		// Example pack — resolve from examples/ relative to e2e file or cwd.
		base := r.e2eDir
		candidate := filepath.Join(base, "examples", spec.Init.Pack, spec.Init.Example)
		if fileio.CheckLocal(candidate) != nil {
			// Try from cwd
			cwd, _ := os.Getwd()
			candidate = filepath.Join(cwd, "examples", spec.Init.Pack, spec.Init.Example)
		}
		r.blueprintFile = filepath.Join(candidate, registry.FileBlueprint)
		r.crFiles = []string{filepath.Join(candidate, registry.FileCR)}

	default:
		// Without a blueprint the test is plain Kubernetes: its CRs, if any,
		// are still applied (cr) and deleted (cr-deleted).
		if spec.Blueprint != "" {
			r.blueprintFile = r.abs(spec.Blueprint)
		}
		for _, p := range spec.AllCRPaths() {
			r.crFiles = append(r.crFiles, r.abs(p))
		}
	}

	if r.blueprintFile != "" {
		if fileio.CheckLocal(r.blueprintFile) != nil {
			return fmt.Errorf("blueprint file not found: %s", r.blueprintFile)
		}
	}
	for _, p := range r.crFiles {
		if fileio.CheckLocal(p) != nil {
			return fmt.Errorf("CR file not found: %s", p)
		}
	}
	return nil
}

func (r *Runner) abs(path string) string {
	abs, err := toAbs(path, r.e2eDir)
	if err != nil {
		return path
	}
	return abs
}
