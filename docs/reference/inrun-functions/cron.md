# Cron Notes

Cron notes work with cron schedule expressions. They cover three problems: extracting individual fields from a string expression, converting between string and structured-map shapes, and producing human-readable descriptions.

## Reference

| Note | Description |
|------|-------------|
| `cronFromMap` | Convert a schedule **map** to a five-field cron string. |
| `cronFromAny` | Convert a schedule value to a five-field cron string. |
| `cronToMap` | Convert a cron string to the structured map shape. |
| `cronNormalize` | Normalize a cron string: expand `@`-macros, trim whitespace, ensure exactly five fields. |
| `cronDescribe` | Return a human-readable description of a cron expression. |
| `cronValid` |  |
| `cronExpr` | Build a five-field cron expression from five explicit string parts. |
| `cronMinute` | Extract a single field by position from a cron string. |
| `cronHour` | Extract a single field by position from a cron string. |
| `cronDom` | Extract a single field by position from a cron string. |
| `cronMonth` | Extract a single field by position from a cron string. |
| `cronDow` | Extract a single field by position from a cron string. |
| `cronField` | Extract a field by index (0–4). |

## Examples

```yaml
# cronFromMap
# onReconcile Path B — input is guaranteed a map by the when: gate
- name: "{{ .metadata.name }}"
  schedule: "{{ cronFromMap .spec.schedule }}"
  when:
    - field: spec.schedule
      typeOf: map
{minute: "*/5", hour: "0", dayOfMonth: "*", month: "*", dayOfWeek: "1"}
→ "*/5 0 * * 1"

# cronFromAny
# normalize block — user input may be either shape
normalize:
  spec:
    schedule: "{{ cronFromAny .spec.schedule }}"

# v2 → v1 conversion — legacy v2 objects may have a flat string
- from: v2
  to: v1
  spec:
    schedule: "{{ cronFromAny .spec.schedule }}"

# status field — safe regardless of stored shape
- path: scheduleExpression
  value: "{{ cronFromAny .spec.schedule }}"
{minute: "*/5", hour: "0", dayOfMonth: "*", month: "*", dayOfWeek: "1"}
→ "*/5 0 * * 1"

"*/5 0 * * 1"
→ "*/5 0 * * 1"   (string passthrough, normalised)
# Old:
schedule: >
  {{ if typeMap .spec.schedule }}{{ cronFromMap .spec.schedule }}
  {{ else }}{{ cronNormalize .spec.schedule }}{{ end }}

# New:
schedule: "{{ cronFromAny .spec.schedule }}"

# cronToMap
# v1 → v2 conversion path
- from: v1
  to: v2
  spec:
    schedule: "{{ cronToMap .spec.schedule }}"
"*/5 0 * * 1"
→ {minute:"*/5", hour:"0", dayOfMonth:"*", month:"*", dayOfWeek:"1"}

"@hourly"
→ {minute:"0", hour:"*", dayOfMonth:"*", month:"*", dayOfWeek:"*"}

# cronNormalize
# value: "{{ cronNormalize .spec.schedule }}"
# "@daily"      → "0 0 * * *"
# "@hourly"     → "0 * * * *"
# "*/5 * * * *" → "*/5 * * * *"  (unchanged, already valid)
# ""            → "* * * * *"

# cronDescribe
- path: scheduleDescription
  value: "{{ cronDescribe .spec.schedule }}"

# cronExpr
# value: "{{ cronExpr .spec.schedule.minute .spec.schedule.hour .spec.schedule.dayOfMonth .spec.schedule.month .spec.schedule.dayOfWeek }}"
# minute="*/1", hour="*", dom="*", month="*", dow="*" → "*/1 * * * *"

# cronMinute
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"

# cronHour
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"

# cronDom
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"

# cronMonth
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"

# cronDow
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"

# cronField
# value: "{{ cronField .spec.schedule 0 }}"   → minute field
# value: "{{ cronField .spec.schedule 3 }}"   → month field
```
