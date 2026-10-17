package note

import (
	"fmt"
	"math"
	"text/template"
	"time"

	"github.com/intentd/inrun/pkg/utils"
	"github.com/robfig/cron/v3"
)

// Time notes
// - timeAgo
// - timeSince
// - timeUntil
// - isExpired
// - timeFormat
// - weekday / weekend
// - timeInWindow / timeNotInWindow
// - nextCron

func timeNotes() template.FuncMap {
	return template.FuncMap{
		"timeAgo":         noteTimeAgo,
		"timeSince":       noteTimeSince,
		"timeUntil":       noteTimeUntil,
		"isExpired":       noteIsExpired,
		"timeFormat":      noteTimeFormat,
		"durationSeconds": noteDurationSeconds,
		"durationAdd":     noteDurationAdd,
		"durationValid":   noteDurationValid,
		"weekday":         noteWeekday,
		"weekend":         noteWeekend,
		"timeInWindow":    noteTimeInWindow,
		"timeNotInWindow": noteTimeNotInWindow,
		"nextCron":        noteNextCron,
	}
}

// ── Time notes ────────────────────────────────────────────────────────────────

// parseTime attempts to parse a timestamp string in RFC3339 or common formats.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// noteTimeAgo returns a human-readable "X ago" string from a timestamp.
//
//	{{ timeAgo .children.cronjob.status.lastScheduleTime }}
//	→ "5m ago" or "2h ago" or "3d ago"
func noteTimeAgo(s string) string {
	t, ok := parseTime(fmt.Sprint(s))
	if !ok {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// noteTimeSince returns the number of seconds elapsed since the timestamp.
//
//	{{ timeSince .metadata.creationTimestamp }}  → 3600
func noteTimeSince(s string) int64 {
	t, ok := parseTime(fmt.Sprint(s))
	if !ok {
		return 0
	}
	return int64(math.Round(time.Since(t).Seconds()))
}

// noteTimeUntil returns a Go duration string representing the time remaining
// until the given timestamp. Returns "0s" when the timestamp is in the past
// or unparseable. Intended for use in requeue.after: to schedule the next
// reconcile at exactly the right moment.
//
//	{{ timeUntil .status.certExpiry }}  → "719h43m12s"
//	{{ timeUntil .status.certExpiry }}  → "0s"  (already expired)
func noteTimeUntil(s string) string {
	t, ok := parseTime(fmt.Sprint(s))
	if !ok {
		return "0s"
	}
	d := time.Until(t)
	if d <= 0 {
		return "0s"
	}
	return d.Truncate(time.Second).String()
}

// noteIsExpired returns true when the timestamp plus the given duration is in the past.
// The duration string follows utils.ParseTimeDuration format — Go's units
// (s, m, h) plus d/w/mo/y (e.g. "30m", "24h", "7d", "1y").
//
// Primary use: declarative rotation checks in when: conditions.
//
//	{{ isExpired (index .metadata.annotations "inrun.dev/generated-at") "30d" }}
//	→ true when the annotation timestamp is more than 30 days old
func noteIsExpired(timestamp interface{}, duration string) bool {
	t, ok := parseTime(fmt.Sprint(timestamp))
	if !ok {
		return false
	}
	d, err := utils.ParseTimeDuration(duration)
	if err != nil {
		return false
	}
	return time.Now().After(t.Add(d))
}

// noteTimeFormat reformats a timestamp string into a human-readable form.
// layout follows Go's time format convention ("Jan 2, 2006", "15:04", etc.)
//
//	{{ timeFormat .metadata.creationTimestamp "Jan 2, 2006" }}
//	→ "Apr 13, 2026"
func noteTimeFormat(timestamp interface{}, layout string) string {
	t, ok := parseTime(fmt.Sprint(timestamp))
	if !ok {
		return ""
	}
	return t.UTC().Format(layout)
}

// noteDurationSeconds parses an extended duration string (Go units plus
// d/w/mo/y) and returns the total seconds as int64. Safe zero value (0) for
// empty or invalid input.
//
//	{{ durationSeconds "5m" }}    → 300
//	{{ durationSeconds "1h30m" }} → 5400
//	{{ durationSeconds "7d" }}    → 604800
func noteDurationSeconds(d string) int64 {
	if d == "" {
		return 0
	}
	dur, err := utils.ParseTimeDuration(d)
	if err != nil {
		return 0
	}
	return int64(dur.Seconds())
}

// noteDurationAdd adds two extended duration strings (Go units plus d/w/mo/y)
// and returns the result as a canonical Go duration string. Safe zero value
// ("0s") for invalid input.
//
//	{{ durationAdd "5m" "30s" }}  → "5m30s"
//	{{ durationAdd "1h" "90m" }}  → "2h30m0s"
//	{{ durationAdd "1d" "12h" }}  → "36h0m0s"
func noteDurationAdd(a, b string) string {
	da, err := utils.ParseTimeDuration(a)
	if err != nil {
		return "0s"
	}
	db, err := utils.ParseTimeDuration(b)
	if err != nil {
		return "0s"
	}
	return (da + db).String()
}

// noteDurationValid reports whether s is a valid extended duration string
// (Go units plus d/w/mo/y). Useful in validation rules.
//
//	{{ durationValid "5m" }}   → true
//	{{ durationValid "5d" }}   → true
//	{{ durationValid "" }}     → false
func noteDurationValid(s string) bool {
	if s == "" {
		return false
	}
	_, err := utils.ParseTimeDuration(s)
	return err == nil
}

// noteWeekday returns true when the current day (UTC) is Monday through Friday.
//
//	{{ weekday }}  → true on a Tuesday, false on a Saturday
func noteWeekday() bool {
	wd := time.Now().UTC().Weekday()
	return wd >= time.Monday && wd <= time.Friday
}

// noteWeekend returns true when the current day (UTC) is Saturday or Sunday.
//
//	{{ weekend }}  → true on a Sunday, false on a Wednesday
func noteWeekend() bool {
	wd := time.Now().UTC().Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// noteTimeInWindow returns true when the current UTC time falls within the
// window [after, before). Both arguments must be "HH:MM" strings.
// Returns false for malformed input.
//
//	{{ timeInWindow "09:00" "18:00" }}  → true at 14:30 UTC, false at 22:00 UTC
func noteTimeInWindow(after, before string) bool {
	now := time.Now().UTC()
	a, err := parseHHMMNote(after, now)
	if err != nil {
		return false
	}
	b, err := parseHHMMNote(before, now)
	if err != nil {
		return false
	}
	return !now.Before(a) && now.Before(b)
}

// noteTimeNotInWindow returns true when the current UTC time is outside the
// window [after, before). It is the exact complement of timeInWindow.
//
//	{{ timeNotInWindow "02:00" "04:00" }}  → true at 10:00 UTC (maintenance window closed)
func noteTimeNotInWindow(after, before string) bool {
	return !noteTimeInWindow(after, before)
}

// noteNextCron returns the next scheduled fire time for a standard 5-field cron
// expression as an RFC3339 string. Returns "" for invalid expressions.
//
//	{{ nextCron "0 9 * * 1" }}  → "2026-07-14T09:00:00Z"  (next Monday 09:00 UTC)
//	{{ nextCron "0 2 * * 0" }}  → next Sunday 02:00 UTC
func noteNextCron(expr string) string {
	schedule, err := cron.ParseStandard(expr)
	if err != nil {
		return ""
	}
	return schedule.Next(time.Now().UTC()).UTC().Format(time.RFC3339)
}

// parseHHMMNote parses a "HH:MM" string anchored to the date of now (UTC).
func parseHHMMNote(s string, now time.Time) (time.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC), nil
}
