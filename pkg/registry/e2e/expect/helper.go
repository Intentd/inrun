package expect

import (
	"errors"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal         = fileio.ReadLocal
	strictUnmarshal   = structured.StrictUnmarshal
	writeFile         = structured.WriteFile
	stringSliceEqual  = structured.StringSliceEqual
	successMark       = term.SuccessMark
	failureMark       = term.FailureMark
	parseTimeDuration = core.ParseTimeDuration
	applyExtract      = types.ApplyExtract
)

// errMaxFailures is returned when the run reached --max-failures or a step
// declared onFailure.stop. The remaining expectations do not run; the result
// collected so far is still reported.
var errMaxFailures = errors.New("stopping after failure")
