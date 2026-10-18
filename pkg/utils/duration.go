package utils

import (
	"time"

	"github.com/intentd/kel/core"
)

// ValidateCronField validates a single cron field value against its allowed range.
func ValidateCronField(field string, min, max int) bool {
	return core.ValidateCronField(field, min, max)
}

// ExpandCronMacro converts @-style macros to five-field expressions.
func ExpandCronMacro(expr string) string {
	return core.ExpandCronMacro(expr)
}

// IsValidCronExpr reports whether expr is a valid cron expression.
func IsValidCronExpr(expr string) bool {
	return core.CronValid(expr)
}

func ParseTimeDuration(s string) (time.Duration, error) {
	return core.ParseTimeDuration(s)
}
