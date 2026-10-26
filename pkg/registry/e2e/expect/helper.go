package expect

import (
	"errors"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/registry/e2e/post"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal           = fileio.ReadLocal
	strictUnmarshal     = structured.StrictUnmarshal
	writeFile           = structured.WriteFile
	stringSliceEqual    = structured.StringSliceEqual
	successMark         = term.SuccessMark
	failureMark         = term.FailureMark
	parseTimeDuration   = core.ParseTimeDuration
	applyExtract        = types.ApplyExtract
	runKubectl          = kubectl.Run
	checkKubectl        = kubectl.Check
	runKubectlMutations = kubectl.Mutate
	runOnFailure        = post.RunOnFailure
	runFinally          = post.RunFinally
)

// errMaxFailures is returned when the run reached --max-failures or a step
// declared onFailure.stop. The remaining expectations do not run; the result
// collected so far is still reported.
var errMaxFailures = errors.New("stopping after failure")

func names(expects []types.E2EExpectation) []string {
	out := make([]string, len(expects))
	for i, e := range expects {
		out[i] = e.Name
	}
	return out
}
