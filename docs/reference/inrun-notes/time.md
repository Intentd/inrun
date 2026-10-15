# Time Notes

Parse timestamps, measure elapsed time, and validate duration strings in Catalog templates. Primarily used in `status.fields` to surface human-readable time values and in `when:` conditions to drive time-based rotation and expiry logic.

All timestamp notes accept RFC3339, RFC3339Nano, `2006-01-02T15:04:05Z`, and `YYYY-MM-DD` formats. Safe zero values are returned for unparseable input.

## Reference

| Note | Description |
|------|-------------|
| `timeAgo` | Return a human-readable elapsed-time string from a timestamp. |
| `timeSince` | Return the number of seconds elapsed since a timestamp as an integer. |
| `timeUntil` | Return a Go duration string representing the time remaining until a timestamp. |
| `isExpired` | Return `true` when a timestamp plus a duration is in the past. |
| `timeFormat` | Reformat a timestamp string using Go's time format layout. |
| `durationSeconds` | Parse an extended duration string (Go units plus `d`/`w`/`mo`/`y`) and return the total number of seconds as an integer. |
| `durationAdd` | Add two extended duration strings (Go units plus `d`/`w`/`mo`/`y`) and return the result as a canonical Go duration string. |
| `durationValid` | Return `true` when the string is a valid extended duration (Go units plus `d`/`w`/`mo`/`y`). |
| `weekday` | Return `true` when the current UTC day is Monday through Friday. |
| `weekend` | Return `true` when the current UTC day is Saturday or Sunday. |
| `timeInWindow` | Return `true` when the current UTC time falls within the window `[after, before)`. |
| `timeNotInWindow` | Return `true` when the current UTC time is outside the window `[after, before)`. |
| `nextCron` | Return the next scheduled fire time for a standard 5-field cron expression as an RFC3339 string. |

## Examples

```yaml
# timeAgo
status:
  fields:
    - path: lastSyncAgo
      value: "{{ timeAgo .children.cronjob.status.lastScheduleTime }}"
      # → "5s ago" / "12m ago" / "3h ago" / "2d ago"

    - path: createdAgo
      value: "{{ timeAgo .metadata.creationTimestamp }}"

# timeSince
# Gate rotation on time elapsed (30 days = 2592000 seconds)
when:
  - field: "{{ timeSince (index .metadata.annotations \"myorg.io/last-rotated\") }}"
    min: "2592000"

# timeUntil
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

# isExpired
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

# timeFormat
status:
  fields:
    - path: createdDate
      value: "{{ timeFormat .metadata.creationTimestamp \"Jan 2, 2006\" }}"
      # → "Apr 13, 2026"

    - path: expiresAt
      value: "{{ timeFormat .status.expirationTime \"2006-01-02\" }}"

# durationSeconds
status:
  fields:
    - path: resyncIntervalSeconds
      value: "{{ durationSeconds .spec.resyncInterval }}"
      # "5m" → 300  |  "1h30m" → 5400  |  "7d" → 604800

# durationAdd
# Compute the total window from base + buffer
status:
  fields:
    - path: totalWindow
      value: "{{ durationAdd .spec.baseWindow .spec.buffer }}"
      # "5m" + "30s" → "5m30s"

# durationValid
spec:
  crds:
    myApp:
      validate:
        - message: "spec.rotationPeriod must be a valid duration (e.g. 30d, 90m, 1y)"
          deny:
            - field: "{{ durationValid .spec.rotationPeriod }}"
              equals: "false"

# weekday
status:
  fields:
    - path: phase
      value: "Active"
      when:
        - field: "{{ weekday }}"
          equals: "true"
    - path: phase
      value: "Suspended"

# weekend
status:
  fields:
    - path: phase
      value: "Weekend"
      when:
        - field: "{{ weekend }}"
          equals: "true"

# timeInWindow
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

# timeNotInWindow
status:
  fields:
    - path: maintenanceActive
      value: "true"
      when:
        - field: "{{ timeNotInWindow \"02:00\" \"04:00\" }}"
          equals: "false"

# nextCron
status:
  fields:
    - path: nextMaintenance
      value: "{{ nextCron \"0 2 * * 0\" }}"
      # → "2026-07-19T02:00:00Z"  (next Sunday 02:00 UTC)

    - path: nextDeploy
      value: "{{ nextCron \"0 9 * * 1\" }}"
      # → next Monday 09:00 UTC
```
