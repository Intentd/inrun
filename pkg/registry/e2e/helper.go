package e2e

import (
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

var (
	readLocal         = utils.ReadLocal
	strictUnmarshal   = utils.StrictUnmarshal
	startSpinner      = utils.StartSpinner
	successMark       = utils.SuccessMark
	failureMark       = utils.FailureMark
	parseTimeDuration = utils.ParseTimeDuration
)

// assertionListMsg is appended to every "at least one assertion required"
// validation error; the names come from types.E2EAssertionNames.
var assertionListMsg = "at least one assertion required (" + strings.Join(types.E2EAssertionNames(), ", ") + ")"
