package runtime

import "github.com/intentd/inrun/pkg/blueprint/validate/internal/check"

// Shared helpers, under the names the checks use.
var (
	failureMark            = check.FailureMark
	handleValidationErrors = check.HandleValidationErrors
	parseTimeDuration      = check.ParseTimeDuration
)
