package e2e

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/registry/e2e/post"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal             = fileio.ReadLocal
	toAbs                 = fileio.ToAbs
	strictUnmarshal       = structured.StrictUnmarshal
	startSpinner          = term.StartSpinner
	successMark           = term.SuccessMark
	failureMark           = term.FailureMark
	parseTimeDuration     = core.ParseTimeDuration
	currentKubectlContext = kubectl.CurrentContext
	runKubectl            = kubectl.Run
	runOnFailure          = post.RunOnFailure
)

// resolveSource resolves the blueprint and CR file paths from the spec.
func (r *Runner) resolveSource() error {
	spec := r.e2e.Spec

	switch {
	case spec.Init != nil:
		// Example pack — resolve from examples/ relative to e2e file or cwd.
		base := r.e2eDir
		candidate := filepath.Join(base, "examples", spec.Init.Pack, spec.Init.Example)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			// Try from cwd
			cwd, _ := os.Getwd()
			candidate = filepath.Join(cwd, "examples", spec.Init.Pack, spec.Init.Example)
		}
		r.blueprintFile = filepath.Join(candidate, registry.FileBlueprint)
		r.crFiles = []string{filepath.Join(candidate, registry.FileCR)}

	case spec.Blueprint != "" && len(spec.AllCRPaths()) > 0:
		r.blueprintFile = r.abs(spec.Blueprint)
		for _, p := range spec.AllCRPaths() {
			r.crFiles = append(r.crFiles, r.abs(p))
		}

	case spec.Blueprint != "":
		// Blueprint without a CR: the CRs come from the gateway (an intent sent
		// by an expectation) or from the expectations themselves.
		r.blueprintFile = r.abs(spec.Blueprint)

	case spec.Custom != nil && spec.Custom.Target != "":
		// custom.target: both blueprint and cr are optional.
		for _, p := range spec.AllCRPaths() {
			r.crFiles = append(r.crFiles, r.abs(p))
		}

	case len(r.e2e.Imports) > 0:
		// Pure aggregator — no own blueprint/CR, just orchestrates imports.
		return nil
	case spec.Cluster.Provider == types.ProviderNone:
		// Running e2e without a cluster: for just commands: useful for testing
		return nil

	default:
		return fmt.Errorf("e2e spec must declare a blueprint, init or custom target, or have imports")
	}

	if r.blueprintFile != "" {
		if _, err := os.Stat(r.blueprintFile); err != nil {
			return fmt.Errorf("blueprint file not found: %s", r.blueprintFile)
		}
	}
	for _, p := range r.crFiles {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("CR file not found: %s", p)
		}
	}
	return nil
}

// isPureAggregator reports whether this E2E exists only to run imported
// files: it has imports, and none of its own — no blueprint, no CR, no
// custom target, no expectations.
func (r *Runner) isPureAggregator() bool {
	if len(r.e2e.Imports) == 0 {
		return false
	}
	return r.blueprintFile == "" &&
		len(r.crFiles) == 0 &&
		!r.kubernetesTarget &&
		len(r.e2e.Spec.Expect) == 0
}

func (r *Runner) abs(path string) string {
	abs, err := toAbs(path, r.e2eDir)
	if err != nil {
		return path
	}
	return abs
}
