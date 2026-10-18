package e2e

import (
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal         = fileio.ReadLocal
	strictUnmarshal   = structured.StrictUnmarshal
	startSpinner      = term.StartSpinner
	successMark       = term.SuccessMark
	failureMark       = term.FailureMark
	parseTimeDuration = core.ParseTimeDuration
)

// assertionListMsg is appended to every "at least one assertion required"
// validation error; the names come from types.E2EAssertionNames.
var assertionListMsg = "at least one assertion required (" + strings.Join(types.E2EAssertionNames(), ", ") + ")"
