package note

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// CronMapSentinel is a prefix written by the cronToMap template function
// to signal that the resolved string is a JSON-encoded cron map, not a plain
// string. Detected by pkg/webhook/conversion_logic.resolveValue, which parses
// it back to map[string]interface{} so it lands as a nested object in the
// converted spec — enabling {{ cronToMap .spec.schedule }} as a one-liner
// shorthand for the v1 → v2 conversion path.
//
// Null bytes are used because they cannot appear in normal YAML field values.
const CronMapSentinel = "\x00CMAP\x00"

func cronNotes() template.FuncMap {
	return template.FuncMap{
		"cronMinute":    cronMinute,
		"cronHour":      cronHour,
		"cronDom":       cronDom,
		"cronMonth":     cronMonth,
		"cronDow":       cronDow,
		"cronField":     cronField,
		"cronExpr":      cronExpr,
		"cronValid":     cronValid,
		"cronFromMap":   cronFromMapStrict,
		"cronToMap":     cronToMapTemplate,
		"cronFromAny":   cronFromMapAny,
		"cronNormalize": cronNormalize,
		"cronDescribe":  cronDescribe,
	}
}

// cronMinute extracts the minute field (position 0) from a cron expression.
// Handles @-macros. Returns "*" for empty input.
//
//	{{ cronMinute "*/5 * * * *" }}   →  "*/5"
//	{{ cronMinute "@hourly" }}        →  "0"
//	{{ cronMinute .spec.schedule }}
func cronMinute(expr string) (string, error) { return cronField(expr, 0) }

// cronHour extracts the hour field (position 1) from a cron expression.
//
//	{{ cronHour "0 2 * * *" }}   →  "2"
//	{{ cronHour "@daily" }}       →  "0"
func cronHour(expr string) (string, error) { return cronField(expr, 1) }

// cronDom extracts the day-of-month field (position 2).
//
//	{{ cronDom "0 0 15 * *" }}   →  "15"
func cronDom(expr string) (string, error) { return cronField(expr, 2) }

// cronMonth extracts the month field (position 3).
//
//	{{ cronMonth "0 0 1 6 *" }}  →  "6"
func cronMonth(expr string) (string, error) { return cronField(expr, 3) }

// cronDow extracts the day-of-week field (position 4).
//
//	{{ cronDow "0 0 * * 1" }}    →  "1"
//	{{ cronDow "@weekly" }}       →  "0"
func cronDow(expr string) (string, error) { return cronField(expr, 4) }

// cronField extracts a single field from a cron expression by position (0-4).
// Expands @-macros before splitting. Returns "*" for empty input.
//
//	{{ cronField .spec.schedule 0 }}   →  minute field
func cronField(expr string, pos int) (string, error) {
	if strings.TrimSpace(expr) == "" {
		return "*", nil
	}
	if pos < 0 || pos > 4 {
		return "*", fmt.Errorf("cronField: position %d out of range [0-4]", pos)
	}
	expanded := expandCronMacro(expr)
	parts := strings.Fields(expanded)
	if len(parts) != 5 {
		return "*", fmt.Errorf(
			"cronField: expected 5 fields in %q, got %d",
			expr, len(parts),
		)
	}
	return parts[pos], nil
}

// cronExpr reconstructs a standard cron expression from five named parts.
// Empty parts default to "*". Used in v2→v1 conversion paths.
//
//	{{ cronExpr .spec.schedule.minute .spec.schedule.hour .spec.schedule.dayOfMonth .spec.schedule.month .spec.schedule.dayOfWeek }}
//	→  "*/1 * * * *"
func cronExpr(minute, hour, dom, month, dow string) string {
	return fmt.Sprintf("%s %s %s %s %s",
		starIfEmpty(minute), starIfEmpty(hour),
		starIfEmpty(dom), starIfEmpty(month), starIfEmpty(dow),
	)
}

// cronValid reports whether expr is a valid cron expression.
//
// Validates that the expression has 5 fields and that each field's values
// are within the correct range. Supports standard cron syntax including
// "*" (all), "*/5" (steps), "1-5" (ranges), "1,2,3" (lists), and macros
// (@hourly, @daily, @weekly, @monthly, @yearly).
//
//	{{ cronValid .spec.schedule }}
func cronValid(expr string) bool {
	return isValidCronExpr(expr)
}

func starIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "*"
	}
	return s
}

// cronFromMapStrict reconstructs a five-field cron expression from a schedule map.
// Errors if the input is not a map — use cronFromAny when the input may be a string.
//
//	{{ cronFromMap .spec.schedule }}  →  "*/5 0 * * 1"
func cronFromMapStrict(v interface{}) (string, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("cronFromMap: expected map, got %T — use cronFromAny if the input may be a cron string", v)
	}
	return cronFromMap(m), nil
}

// cronFromMapAny reconstructs a five-field cron expression from either a schedule
// map or a cron string. Safe zero value ("* * * * *") for nil or unknown input.
//
//	{{ cronFromAny .spec.schedule }}  →  "*/5 0 * * 1"
func cronFromMapAny(v interface{}) string {
	switch m := v.(type) {
	case map[string]interface{}:
		return cronFromMap(m)
	case string:
		return cronNormalize(m)
	default:
		return "* * * * *"
	}
}

// CronToMap is the Go API for splitting a cron expression into its five named
// fields. Accepts a string or an existing map (returned as-is).
func CronToMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	expr := fmt.Sprintf("%v", v)
	out := map[string]interface{}{
		"minute": "*", "hour": "*", "dayOfMonth": "*", "month": "*", "dayOfWeek": "*",
	}
	if strings.TrimSpace(expr) == "" {
		return out
	}
	expanded := expandCronMacro(expr)
	parts := strings.Fields(expanded)
	if len(parts) != 5 {
		return out
	}
	out["minute"] = parts[0]
	out["hour"] = parts[1]
	out["dayOfMonth"] = parts[2]
	out["month"] = parts[3]
	out["dayOfWeek"] = parts[4]
	return out
}

// cronToMapTemplate is the FuncMap registration for cronToMap.
// It serialises the result as CronMapSentinel + JSON so that resolveValue in
// pkg/webhook/conversion_logic can detect it and return a proper
// map[string]interface{} instead of a flat string — enabling
// {{ cronToMap .spec.schedule }} as a one-liner in conversion path specs.
//
//	{{ cronToMap .spec.schedule }}  →  (detected by resolveValue → map)
func cronToMapTemplate(v interface{}) (string, error) {
	m := CronToMap(v)
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("cronToMap: %w", err)
	}
	return CronMapSentinel + string(b), nil
}

// cronFromMap is kept as the unexported Go helper used internally.
func cronFromMap(m map[string]interface{}) string {
	get := func(key string) string {
		if v, ok := m[key]; ok {
			s := fmt.Sprintf("%v", v)
			if strings.TrimSpace(s) != "" {
				return s
			}
		}
		return "*"
	}
	return cronExpr(
		get("minute"),
		get("hour"),
		get("dayOfMonth"),
		get("month"),
		get("dayOfWeek"),
	)
}

// cronNormalize — canonical formatting
// This ensures:
// macros expanded
// whitespace trimmed
// missing fields → *
// output always 5 fields
// no surprises
func cronNormalize(expr string) string {
	if strings.TrimSpace(expr) == "" {
		return "* * * * *"
	}

	expanded := expandCronMacro(expr)
	parts := strings.Fields(expanded)

	// If invalid, normalize to all "*"
	if len(parts) != 5 {
		return "* * * * *"
	}

	// Trim each field
	for i := range parts {
		if strings.TrimSpace(parts[i]) == "" {
			parts[i] = "*"
		}
	}

	return strings.Join(parts, " ")
}

// cronDescribe — human‑readable explanation
// This is the fun one.
// It turns:
//
//	*/5 * * * *
//		into:
//	Every 5 minutes
//
// And:
//
//	0 2 * * 1
//		into:
//	At 02:00 on Mondays
func cronDescribe(expr string) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return "Invalid cron expression"
	}
	// Reject inputs that can't be a cron expression: must be a @macro or have 5 fields.
	if !strings.HasPrefix(trimmed, "@") && len(strings.Fields(trimmed)) != 5 {
		return "Invalid cron expression"
	}
	norm := cronNormalize(trimmed)
	parts := strings.Fields(norm)
	if len(parts) != 5 {
		return "Invalid cron expression"
	}

	min, hr, dom, mon, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Minute-only interval
	if strings.HasPrefix(min, "*/") && hr == "*" && dom == "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("Every %s minutes", strings.TrimPrefix(min, "*/"))
	}

	// Hourly
	if min == "0" && hr == "*" && dom == "*" && mon == "*" && dow == "*" {
		return "Every hour"
	}

	// Daily at specific time
	if dom == "*" && mon == "*" && dow == "*" && min != "*" && hr != "*" {
		return fmt.Sprintf("At %s:%s every day", hr, min)
	}

	// Weekly
	if dom == "*" && mon == "*" && dow != "*" {
		return fmt.Sprintf("At %s:%s on day-of-week %s", hr, min, dow)
	}

	// Monthly
	if dom != "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("At %s:%s on day %s of every month", hr, min, dom)
	}

	// Fallback
	return fmt.Sprintf("Cron: %s", norm)
}
