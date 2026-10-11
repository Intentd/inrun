// pkg/types/when.go
//
// EvaluateConditions — extended condition evaluation with OR logic.
//
// The when: field ([]Condition) uses AND semantics.
// or: is a parallel field on template sources with OR semantics.
//
//	# AND only
//	when:
//	  - field: status.phase
//	    equals: "Ready"
//
//	# OR
//	or:
//	  - field: status.phase
//	    equals: "Failed"
//	  - field: status.phase
//	    equals: "Succeeded"
//
//	# Combined: (spec.enabled=true) AND (phase=Failed OR phase=Succeeded)
//	when:
//	  - field: spec.enabled
//	    equals: "true"
//	or:
//	  - field: status.phase
//	    equals: "Failed"
//	  - field: status.phase
//	    equals: "Succeeded"
package types

import (
	"fmt"
	"github.com/inrundev/inrun/pkg/utils"
	"regexp"
	"strings"
	"time"

	"github.com/inrundev/inrun/pkg/note"
	"github.com/robfig/cron/v3"
)

// TemplateEvaluator evaluates a Go template expression against the current
// reconcile data map, including all note functions and enriched children.
// Returns (result, true) on success; ("", false) on any error.
// Implemented by the resolver package and injected at call sites.
// nil disables template evaluation — all existing callers are backward-compatible.
type TemplateEvaluator func(tmpl string) (string, bool)

// IsTemplate reports whether s contains a Go template expression.
func IsTemplate(s string) bool { return strings.Contains(s, "{{") }

// EvaluateConditions evaluates when: (allOf, AND) and or: (OR) conditions.
// data is resolver.Data() — full CR map including children, external, cross.
// eval is optional — pass nil to disable template evaluation (backward compatible).
//
// Both blocks must pass when both are declared.
// Empty blocks always pass.
func EvaluateConditions(data map[string]interface{}, allOf []Condition, or []Condition, eval TemplateEvaluator) bool {
	for _, cond := range allOf {
		if !EvaluateOneCond(data, cond, eval) {
			return false
		}
	}
	if len(or) > 0 {
		passed := false
		for _, cond := range or {
			if EvaluateOneCond(data, cond, eval) {
				passed = true
				break
			}
		}
		if !passed {
			return false
		}
	}
	return true
}

// EvaluateOneCond evaluates a single Condition against a data map.
// Exported so the template package and reconciler package can both call it.
// Defined here in pkg/types to avoid import cycles.
//
// When cond.Field is a template expression AND eval is non-nil, the expression
// is evaluated and the string result is used for operator comparison.
// Time-based conditions (time:, dayOfWeek:, cron:) are evaluated against the
// current wall clock — they do not reference the data map.
func EvaluateOneCond(data map[string]interface{}, cond Condition, eval TemplateEvaluator) bool {
	result := evaluateOneCond(data, cond, eval)
	if cond.Negate {
		return !result
	}
	return result
}

func evaluateOneCond(data map[string]interface{}, cond Condition, eval TemplateEvaluator) bool {
	// ── Time-based conditions ─────────────────────────────────────────────────

	if cond.Time != nil {
		return evalTimeWindow(cond.Time, time.Now())
	}

	if cond.DayOfWeek != nil {
		return evalDayOfWeek(cond.DayOfWeek, time.Now())
	}

	if cond.Cron != "" {
		// Prefer caller-injected window state (from TickCronWindow) when available.
		// Callers that manage cron state (autoscaler, job runner) inject it under
		// data["_cronWindows"][cronExpr] = "true"/"false" before calling EvaluateConditions.
		if windows, ok := data["_cronWindows"].(map[string]interface{}); ok {
			if v, ok := windows[cond.Cron]; ok {
				return v == "true"
			}
		}
		// Stateless fallback: window open if a cron fire occurred within duration.
		// When duration is unset, defaults to one natural period of the schedule.
		return evalCronWindow(cond.Cron, cond.Duration.Duration, time.Now())
	}

	op, expected := ResolveConditionOp(cond)

	// ── Template expected-value resolution ────────────────────────────────
	// If the comparison value itself is a template expression, evaluate it so
	// conditions like `equals: "{{ .spec.image }}"` work as intended.
	if IsTemplate(expected) && eval != nil {
		if resolved, ok := eval(expected); ok {
			expected = resolved
		}
	}

	// ── Template field resolution ──────────────────────────────────────────
	// If the field is a template expression, evaluate it through the resolver.
	// The string result is used for operator comparison — same logic as dot path.
	if IsTemplate(cond.Field) && eval != nil {
		result, ok := eval(cond.Field)
		if !ok {
			return false // fail silently — same behaviour as missing dot path
		}
		return applyOperator(op, result, expected, data, cond)
	}

	// ── Standard dot path resolution ──────────────────────────────────────
	fieldVal := NavigateDotPath(data, cond.Field)
	return applyOperator(op, fieldVal, expected, data, cond)
}

// applyOperator applies the resolved operator to fieldVal and expected.
// Shared by both template-expression and dot-path resolution paths.
func applyOperator(op ConditionOperator, fieldVal, expected string, data map[string]interface{}, cond Condition) bool {
	switch op {
	case ConditionExists:
		return fieldVal != "" && fieldVal != "<no value>"
	case ConditionNotExists:
		return fieldVal == "" || fieldVal == "<no value>"
	case ConditionEquals:
		return fieldVal == expected
	case ConditionNotEquals:
		return fieldVal != expected
	case ConditionContains:
		return typeContains(fieldVal, expected)
	case ConditionNotContains:
		return !typeContains(fieldVal, expected)
	case ConditionRegex:
		re, err := regexp.Compile(expected)
		if err != nil {
			return false
		}
		return re.MatchString(fieldVal)
	case ConditionPrefix:
		return typeHasPrefix(fieldVal, expected)
	case ConditionNotPrefix:
		return !typeHasPrefix(fieldVal, expected)
	case ConditionSuffix:
		return typeHasSuffix(fieldVal, expected)
	case ConditionNotSuffix:
		return !typeHasSuffix(fieldVal, expected)
	case ConditionGt:
		fv, _ := typeParseFloat(fieldVal) // absent = 0
		ev, ee := typeParseFloat(expected)
		if ee != nil {
			return false
		}
		return fv > ev
	case ConditionLt:
		fv, _ := typeParseFloat(fieldVal)
		ev, ee := typeParseFloat(expected)
		if ee != nil {
			return false
		}
		return fv < ev
	case ConditionGte:
		fv, _ := typeParseFloat(fieldVal)
		ev, ee := typeParseFloat(expected)
		if ee != nil {
			return false
		}
		return fv >= ev
	case ConditionLte:
		fv, _ := typeParseFloat(fieldVal)
		ev, ee := typeParseFloat(expected)
		if ee != nil {
			return false
		}
		return fv <= ev
	case ConditionIn:
		for _, v := range typesSplitComma(expected) {
			if typesTrimSpace(v) == fieldVal {
				return true
			}
		}
		return false
	case ConditionNotIn:
		for _, v := range typesSplitComma(expected) {
			if typesTrimSpace(v) == fieldVal {
				return false
			}
		}
		return true
	case ConditionBetween:
		lo, hi, ok := parseBetween(expected)
		if !ok {
			return false
		}
		fv, err := typeParseFloat(fieldVal)
		if err != nil {
			return false
		}
		return fv >= lo && fv <= hi
	case ConditionNotBetween:
		lo, hi, ok := parseBetween(expected)
		if !ok {
			return false
		}
		fv, err := typeParseFloat(fieldVal)
		if err != nil {
			return false
		}
		return fv < lo || fv > hi
	case ConditionUnique:
		return resolveUnique(data, cond.Field, fieldVal)
	case ConditionTypeOf:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == expected
	case ConditionTypeMap:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "map"
	case ConditionTypeList:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "slice"
	case ConditionTypeString:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "string"
	case ConditionTypeNumber:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "number"
	case ConditionTypeBool:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "bool"
	case ConditionTypeNull:
		raw := NavigateRawPath(data, cond.Field)
		return note.TypeOf(raw) == "null"
	}
	return false
}

// NavigateRawPath walks a dot-notation path through a nested map.
// Returns interface{} when any segment is missing — the notExists case.
// Exported for use by the template package and status field resolver.
func NavigateRawPath(m map[string]interface{}, path string) interface{} {
	// Empty path check
	if path == "" {
		return nil
	}

	// Start at the root with 'current' as cursor
	var current interface{} = m

	// Split the path into parts(slice)
	for _, part := range typesSplitDot(path) {
		// Ensure current is a map
		typed, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		// Follow the path
		current, ok = typed[part]
		if !ok {
			return nil
		}
	}
	if current == nil {
		return nil
	}

	return current
}

// NavigateDotPath walks a dot-notation path through a nested map.
// Returns "" when any segment is missing — the notExists case.
// Exported for use by the template package and status field resolver.
func NavigateDotPath(m map[string]interface{}, path string) string {
	// Empty path check
	if path == "" {
		return ""
	}

	// Start at the root with 'current' as cursor
	var current interface{} = m

	// Split the path into parts(slice)
	for _, part := range typesSplitDot(path) {
		// Ensure current is a map
		typed, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		// Follow the path
		current, ok = typed[part]
		if !ok {
			return ""
		}
	}
	if current == nil {
		return ""
	}
	if s, ok := current.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", current)
}

// ResolveConditionOp resolves the effective operator and comparison value
// from a Condition, respecting shorthand fields.
// Exported so the template package can use the same resolution logic.
func ResolveConditionOp(c Condition) (ConditionOperator, string) {
	if c.Exists != nil && *c.Exists {
		return ConditionExists, ""
	}
	if c.NotExists != nil && *c.NotExists {
		return ConditionNotExists, ""
	}
	if c.Equals != "" {
		return ConditionEquals, c.Equals
	}
	if c.NotEquals != "" {
		return ConditionNotEquals, c.NotEquals
	}
	if c.Prefix != "" {
		return ConditionPrefix, c.Prefix
	}
	if c.NotPrefix != "" {
		return ConditionNotPrefix, c.NotPrefix
	}
	if c.Suffix != "" {
		return ConditionSuffix, c.Suffix
	}
	if c.NotSuffix != "" {
		return ConditionNotSuffix, c.NotSuffix
	}
	if c.Contains != "" {
		return ConditionContains, c.Contains
	}
	if c.NotContains != "" {
		return ConditionNotContains, c.NotContains
	}
	if c.Regex != "" {
		return ConditionRegex, c.Regex
	}
	if c.GreaterThan != "" {
		return ConditionGt, c.GreaterThan
	}
	if c.LessThan != "" {
		return ConditionLt, c.LessThan
	}
	if c.GreaterThanOrEqual != "" {
		return ConditionGte, c.GreaterThanOrEqual
	}
	if c.LessThanOrEqual != "" {
		return ConditionLte, c.LessThanOrEqual
	}
	if c.Min != "" {
		return ConditionGte, c.Min
	}
	if c.Max != "" {
		return ConditionLte, c.Max
	}
	if c.Between != "" {
		return ConditionBetween, c.Between
	}
	if c.NotBetween != "" {
		return ConditionNotBetween, c.NotBetween
	}
	if c.In != "" {
		return ConditionIn, c.In
	}
	if c.NotIn != "" {
		return ConditionNotIn, c.NotIn
	}
	if c.Operator != "" {
		return c.Operator, c.Value
	}
	if c.Value != "" {
		return ConditionEquals, c.Value
	}
	return ConditionExists, ""
}

// ── Private string/numeric helpers ───────────────────────────────────────────

func typesSplitDot(s string) []string   { return utils.SplitField(s) }
func typesSplitComma(s string) []string { return typesSplitOn(s, ',') }

func typesSplitOn(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func typesTrimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func typeContains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func typeHasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
func typeHasSuffix(s, p string) bool { return len(s) >= len(p) && s[len(s)-len(p):] == p }

// typeParseFloat treats empty string as 0 — Kubernetes omits zero-value integers.
func typeParseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// parseBetween splits a "min,max" comma pair into two floats, both
// inclusive bounds. ok is false unless expected is exactly two
// comma-separated numeric values — shared by EvaluateOneCond and
// EvaluateValidationRule so between/notBetween parse identically in both.
func parseBetween(expected string) (lo, hi float64, ok bool) {
	parts := typesSplitComma(expected)
	if len(parts) != 2 {
		return 0, 0, false
	}
	var err error
	if lo, err = typeParseFloat(typesTrimSpace(parts[0])); err != nil {
		return 0, 0, false
	}
	if hi, err = typeParseFloat(typesTrimSpace(parts[1])); err != nil {
		return 0, 0, false
	}
	return lo, hi, true
}

// ── Time-based condition helpers ──────────────────────────────────────────────

// evalTimeWindow returns true when now is within the declared HH:MM window.
func evalTimeWindow(tw *TimeWindow, now time.Time) bool {
	if tw.After != "" {
		t, err := parseHHMM(tw.After, now)
		if err != nil || now.Before(t) {
			return false
		}
	}
	if tw.Before != "" {
		t, err := parseHHMM(tw.Before, now)
		if err != nil || now.After(t) {
			return false
		}
	}
	return true
}

// EvalDayOfWeekAt is the exported entry point for tests that need a fixed clock.
func EvalDayOfWeekAt(d *DayOfWeekCondition, now time.Time) bool {
	return evalDayOfWeek(d, now)
}

// evalDayOfWeek returns true when today matches the declared day constraint.
func evalDayOfWeek(d *DayOfWeekCondition, now time.Time) bool {
	wd := now.Weekday()
	if d.Weekday != nil {
		isWeekday := wd >= time.Monday && wd <= time.Friday
		return *d.Weekday == isWeekday
	}
	if d.Weekend != nil {
		isWeekend := wd == time.Saturday || wd == time.Sunday
		return *d.Weekend == isWeekend
	}
	today := wd.String()
	if len(d.In) > 0 {
		for _, day := range d.In {
			if strings.EqualFold(day, today) {
				return true
			}
		}
		return false
	}
	if len(d.NotIn) > 0 {
		for _, day := range d.NotIn {
			if strings.EqualFold(day, today) {
				return false
			}
		}
		return true
	}
	return false
}

// evalCronWindow returns true when a cron-defined window is currently open.
// The window opens at each cron fire and stays open for duration.
// When duration is zero the window defaults to one natural period of the schedule
// (the interval between two consecutive future fires), so the condition stays open
// from the previous fire until the next one regardless of the reconcile interval.
func evalCronWindow(cronExpr string, duration time.Duration, now time.Time) bool {
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return false
	}
	if duration == 0 {
		// Derive the natural period from two consecutive fires.
		next := schedule.Next(now)
		period := schedule.Next(next).Sub(next)
		duration = period
	}
	// Window is open if a fire occurred within the last duration.
	prev := schedule.Next(now.Add(-duration))
	return !prev.After(now)
}

// parseHHMM parses "HH:MM" and anchors it to today in now's location.
func parseHHMM(s string, now time.Time) (time.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}
