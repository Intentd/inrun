# Time

Reason about timestamps (RFC3339) and durations (Go units plus d, w, mo and y): elapsed and remaining time, formatting, time windows and weekdays, duration arithmetic, and the next run of a cron schedule.

## Reference

| Function | Description |
|----------|-------------|
| [`durationAdd`](#durationadd) | Adds two extended duration strings (Go units plus d/w/mo/y) and returns the result as a canonical Go duration string. |
| [`durationSeconds`](#durationseconds) | Parses an extended duration string (Go units plus d/w/mo/y) and returns the total seconds. |
| [`durationValid`](#durationvalid) | Returns `true` when the string is a valid extended duration (Go units plus `d`/`w`/`mo`/`y`). |
| [`isExpired`](#isexpired) | Returns `true` when a timestamp plus a duration is in the past. |
| [`nextCron`](#nextcron) | Returns the next scheduled fire time for a standard 5-field cron expression as an RFC3339 string. |
| [`timeAgo`](#timeago) | Returns a human-readable elapsed-time string from a timestamp. |
| [`timeFormat`](#timeformat) | Reformats a timestamp string using Go's time format layout. |
| [`timeInWindow`](#timeinwindow) | Returns `true` when the current UTC time falls within the window `[after, before)`. |
| [`timeNotInWindow`](#timenotinwindow) | Returns `true` when the current UTC time is outside the window `[after, before)`. |
| [`timeSince`](#timesince) | Returns the number of seconds elapsed since a timestamp as an integer, for comparing against a threshold. |
| [`timeUntil`](#timeuntil) | Returns a Go duration string representing the time remaining until a timestamp. |
| [`weekday`](#weekday) | Returns `true` when the current UTC day is Monday through Friday. |
| [`weekend`](#weekend) | Returns `true` when the current UTC day is Saturday or Sunday. |

## Functions

### durationAdd

`durationAdd` adds two extended duration strings (Go units plus d/w/mo/y) and returns the result as a canonical Go duration string. An error when either does not parse.

```text
{{ durationAdd "5m" "30s" }}  → "5m30s"
{{ durationAdd "1h" "90m" }}  → "2h30m0s"
{{ durationAdd "1d" "12h" }}  → "36h0m0s"
```

### durationSeconds

`durationSeconds` parses an extended duration string (Go units plus d/w/mo/y) and returns the total seconds. 0 for an empty string; an error for one that does not parse.

```text
{{ durationSeconds "5m" }}    → 300
{{ durationSeconds "1h30m" }} → 5400
{{ durationSeconds "7d" }}    → 604800
```

### durationValid

`durationValid` returns `true` when the string is a valid extended duration (Go units plus `d`/`w`/`mo`/`y`). Use in validation rules to reject malformed duration fields.

```text
{{ durationValid "30d" }}    → true
{{ durationValid "thirty" }}  → false
```

### isExpired

`isExpired` returns `true` when a timestamp plus a duration is in the past. Use it to decide when something is due for rotation or cleanup. Duration is an extended duration string — Go's units (`"30m"`, `"24h"`) plus `d`/`w`/`mo`/`y` (`"30d"`, `"1y"`).

```text
{{ isExpired "2026-01-15T10:30:00Z" "30d" }}  → true
```

### nextCron

`nextCron` returns the next scheduled fire time for a standard 5-field cron expression as an RFC3339 string. An error for an expression that does not parse.

```text
{{ nextCron "0 9 * * 1" }}  → "2026-07-14T09:00:00Z"  (next Monday 09:00 UTC)
{{ nextCron "0 2 * * 0" }}  → next Sunday 02:00 UTC
```

### timeAgo

`timeAgo` returns a human-readable elapsed-time string from a timestamp. Outputs seconds, minutes, hours, or days depending on magnitude.

```text
{{ timeAgo .deployment.metadata.creationTimestamp }}  → "3d ago"
```

### timeFormat

`timeFormat` reformats a timestamp string using Go's time format layout. Returns `""` for unparseable input.

```text
{{ timeFormat "2026-04-13T09:30:00Z" "Jan 2, 2006" }}  → "Apr 13, 2026"
```

### timeInWindow

`timeInWindow` returns `true` when the current UTC time falls within the window `[after, before)`. Both arguments must be `"HH:MM"` strings. Returns `false` for malformed input.

```text
{{ timeInWindow "09:00" "18:00" }}
{{ and (timeInWindow "09:00" "18:00") weekday }}
```

### timeNotInWindow

`timeNotInWindow` returns `true` when the current UTC time is outside the window `[after, before)`. The exact complement of `timeInWindow`. Useful for maintenance-window logic where the resource should exist only when the window is closed.

```text
{{ timeNotInWindow "02:00" "04:00" }}
```

### timeSince

`timeSince` returns the number of seconds elapsed since a timestamp as an integer, for comparing against a threshold.

```text
{{ timeSince .deployment.metadata.creationTimestamp }}  → 259200
```

### timeUntil

`timeUntil` returns a Go duration string representing the time remaining until a timestamp. Returns `"0s"` when the timestamp is in the past or unparseable. Useful as a requeue delay, so a controller wakes up exactly when the timestamp passes.

```text
{{ timeUntil .certificate.status.notAfter }}  → "719h43m12s"
```

### weekday

`weekday` returns `true` when the current UTC day is Monday through Friday. Use it to act only on business days.

```text
{{ if weekday }}business day{{ end }}
```

### weekend

`weekend` returns `true` when the current UTC day is Saturday or Sunday. The exact complement of `weekday`.

```text
{{ if weekend }}scaled down for the weekend{{ end }}
```
