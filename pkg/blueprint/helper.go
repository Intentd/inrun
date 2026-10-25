package blueprint

import (
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"
)

// ── utils aliases ────────────────────────────────────────────────────────────
// Import utils once here. All other files in this package use these names
// directly — no per-file utils import needed.

var (
	failureMark       = term.FailureMark
	warningMark       = term.WarningMark
	parseTimeDuration = core.ParseTimeDuration
	isTemplate        = types.IsTemplate
	readLocal         = fileio.ReadLocal
)
