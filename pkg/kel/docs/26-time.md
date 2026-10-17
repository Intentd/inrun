# 26 — Time Notes

Parse timestamps, measure elapsed time, and validate duration strings in Catalog templates. Primarily used in `status.fields` to surface human-readable time values and in `when:` conditions to drive time-based rotation and expiry logic.

All timestamp notes accept RFC3339, RFC3339Nano, `2006-01-02T15:04:05Z`, and `YYYY-MM-DD` formats. Safe zero values are returned for unparseable input.

## Reference

### `timeAgo`

Return a human-readable elapsed-time string from a timestamp. Outputs seconds, minutes, hours, or days depending on magnitude.

Keywords: time, ago, elapsed, human, readable, display, status, string

```yaml
status:
  fields:
    - path: lastSyncAgo
      value: "{{ timeAgo .children.cronjob.status.lastScheduleTime }}"
      # → "5s ago" / "12m ago" / "3h ago" / "2d ago"

    - path: createdAgo
      value: "{{ timeAgo .metadata.creationTimestamp }}"
```

---

### `timeSince`

Return the number of seconds elapsed since a timestamp as an integer. Use for programmatic comparisons in `when:` conditions.

Keywords: time, since, seconds, elapsed, duration, int, compare

```yaml
# Gate rotation on time elapsed (30 days = 2592000 seconds)
when:
  - field: "{{ timeSince (index .metadata.annotations \"myorg.io/last-rotated\") }}"
    min: "2592000"
```

---

### `timeUntil`

Return a Go duration string representing the time remaining until a timestamp. Returns `"0s"` when the timestamp is in the past or unparseable. The primary use is `requeue.after:` — schedule the next reconcile at exactly the right moment for this specific CR.

Keywords: time, until, remaining, duration, expiry, cert, requeue, schedule

```yaml
operatorBox:
  reconcile:
    requeue:
      after: "{{ timeUntil .status.certExpiry }}"
      when:
        - field: status.certExpiry
          exists: true

status:
  fields:
    - path: timeUntilExpiry
      value: "{{ timeUntil .status.certExpiry }}"
      # → "719h43m12s"  (about 30 days)
      # → "0s"          (already expired)
```

---

### `isExpired`

Return `true` when a timestamp plus a duration is in the past. The canonical way to drive rotation logic in `when:` conditions. Duration is an extended duration string — Go's units (`"30m"`, `"24h"`) plus `d`/`w`/`mo`/`y` (`"30d"`, `"1y"`).

Keywords: time, expired, expiry, rotation, ttl, boolean, when, condition

```yaml
# Recreate the secret when the rotation annotation says it is due
onCreate:
  secrets:
    - name: "{{ .metadata.name }}-token"
      once: true
      rotateAfter: 30d

# Gate a resource on whether a timestamp annotation is past its TTL
when:
  - field: "{{ isExpired (index .metadata.annotations \"myorg.io/generated-at\") \"30d\" }}"
    equals: "true"
```

---

### `timeFormat`

Reformat a timestamp string using Go's time format layout. Returns `""` for unparseable input.

Keywords: time, format, layout, display, readable, string, date

```yaml
status:
  fields:
    - path: createdDate
      value: "{{ timeFormat .metadata.creationTimestamp \"Jan 2, 2006\" }}"
      # → "Apr 13, 2026"

    - path: expiresAt
      value: "{{ timeFormat .status.expirationTime \"2006-01-02\" }}"
```

---

### `durationSeconds`

Parse an extended duration string (Go units plus `d`/`w`/`mo`/`y`) and return the total number of seconds as an integer. Returns `0` for invalid input.

Keywords: time, duration, seconds, parse, int, convert

```yaml
status:
  fields:
    - path: resyncIntervalSeconds
      value: "{{ durationSeconds .spec.resyncInterval }}"
      # "5m" → 300  |  "1h30m" → 5400  |  "7d" → 604800
```

---

### `durationAdd`

Add two extended duration strings (Go units plus `d`/`w`/`mo`/`y`) and return the result as a canonical Go duration string. Returns `"0s"` for invalid input.

Keywords: time, duration, add, combine, string

```yaml
# Compute the total window from base + buffer
status:
  fields:
    - path: totalWindow
      value: "{{ durationAdd .spec.baseWindow .spec.buffer }}"
      # "5m" + "30s" → "5m30s"
```

---

### `durationValid`

Return `true` when the string is a valid extended duration (Go units plus `d`/`w`/`mo`/`y`). Use in validation rules to reject malformed duration fields.

Keywords: time, duration, valid, validate, boolean, check

```yaml
spec:
  crds:
    myApp:
      validate:
        - message: "spec.rotationPeriod must be a valid duration (e.g. 30d, 90m, 1y)"
          deny:
            - field: "{{ durationValid .spec.rotationPeriod }}"
              equals: "false"
```

---

### `weekday`

Return `true` when the current UTC day is Monday through Friday. Use in `when:` conditions or status fields to gate resources on business days.

Keywords: time, weekday, businessday, monday, friday, boolean, window

```yaml
status:
  fields:
    - path: phase
      value: "Active"
      when:
        - field: "{{ weekday }}"
          equals: "true"
    - path: phase
      value: "Suspended"
```

---

### `weekend`

Return `true` when the current UTC day is Saturday or Sunday. The exact complement of `weekday`.

Keywords: time, weekend, saturday, sunday, boolean, offpeak

```yaml
status:
  fields:
    - path: phase
      value: "Weekend"
      when:
        - field: "{{ weekend }}"
          equals: "true"
```

---

### `timeInWindow`

Return `true` when the current UTC time falls within the window `[after, before)`. Both arguments must be `"HH:MM"` strings. Returns `false` for malformed input.

Keywords: time, window, businesshours, after, before, boolean, schedule

```yaml
status:
  fields:
    - path: phase
      value: "Active"
      when:
        - field: "{{ timeInWindow \"09:00\" \"18:00\" }}"
          equals: "true"
    - path: phase
      value: "Suspended"

# Combine with weekday for a full business-hours check:
# {{ and (timeInWindow "09:00" "18:00") weekday }}
```

---

### `timeNotInWindow`

Return `true` when the current UTC time is outside the window `[after, before)`. The exact complement of `timeInWindow`. Useful for maintenance-window logic where the resource should exist only when the window is closed.

Keywords: time, window, maintenance, outside, boolean, complement

```yaml
status:
  fields:
    - path: maintenanceActive
      value: "true"
      when:
        - field: "{{ timeNotInWindow \"02:00\" \"04:00\" }}"
          equals: "false"
```

---

### `nextCron`

Return the next scheduled fire time for a standard 5-field cron expression as an RFC3339 string. Returns `""` for invalid expressions.

Keywords: time, cron, next, schedule, rfc3339, string, transition

```yaml
status:
  fields:
    - path: nextMaintenance
      value: "{{ nextCron \"0 2 * * 0\" }}"
      # → "2026-07-19T02:00:00Z"  (next Sunday 02:00 UTC)

    - path: nextDeploy
      value: "{{ nextCron \"0 9 * * 1\" }}"
      # → next Monday 09:00 UTC
```

---

## Quick reference

| Note | Accepts | Returns | Notes |
|------|---------|---------|-------|
| `timeAgo` | `timestamp string` | `string` | `"Xs ago"` / `"Xm ago"` / `"Xh ago"` / `"Xd ago"` |
| `timeSince` | `timestamp string` | `int64` | seconds since timestamp |
| `timeUntil` | `timestamp string` | `string` | Go duration string until timestamp; `"0s"` if past |
| `isExpired` | `timestamp string, duration string` | `bool` | `true` when `timestamp + duration` is in the past |
| `timeFormat` | `timestamp, layout string` | `string` | Go time layout |
| `durationSeconds` | `duration string` | `int64` | Go duration → seconds |
| `durationAdd` | `a, b string` | `string` | sum of two Go durations |
| `durationValid` | `duration string` | `bool` | `false` for `"5d"` — Go has no days |
| `weekday` | — | `bool` | `true` Mon–Fri UTC |
| `weekend` | — | `bool` | `true` Sat–Sun UTC |
| `timeInWindow` | `after, before string` | `bool` | `true` when now is in `[HH:MM, HH:MM)` UTC |
| `timeNotInWindow` | `after, before string` | `bool` | complement of `timeInWindow` |
| `nextCron` | `expr string` | `string` | next RFC3339 fire time for a 5-field cron |
