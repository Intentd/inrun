package kubectl

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
	toAbs             = fileio.ToAbs
	startSpinner      = term.StartSpinner
	parseTimeDuration = core.ParseTimeDuration
	applyExtract      = types.ApplyExtract
	requireErr        = structured.RequireErr
)

// assertionListMsg is appended to every "at least one assertion required"
// validation error; the names come from types.E2EAssertionNames, plus the
// JSON extraction/assertion blocks that count as assertions on their own.
var assertionListMsg = "at least one assertion required (" + strings.Join(types.E2EAssertionNames(), ", ") + ", json:)"
