# 05 — Cron Notes

Cron notes work with cron schedule expressions. They cover three problems: extracting individual fields from a string expression, converting between string and structured-map shapes, and producing human-readable descriptions.

## The two schedule shapes

Users may declare schedules as a plain string or as a structured object:

```yaml
# String (v1 shape)
spec:
  schedule: "*/5 * * * *"

# Structured map (v2 shape)
spec:
  schedule:
    minute: "*/5"
    hour: "*"
    dayOfMonth: "*"
    month: "*"
    dayOfWeek: "*"
```

`cronFromMap` and `cronToMap` convert between the two shapes. Both notes accept either type — a string or a map — so conversion paths stay correct even when existing objects predate a schema change.

## Cron expression format

Standard five-field cron: `minute hour dayOfMonth month dayOfWeek`

```
*/5 * * * *
 │  │ │   │ └── day of week  (0-7, 0=Sun, 7=Sun, or names)
 │  │ │   └──── month        (1-12 or names)
 │  │ └──────── day of month (1-31)
 │  └────────── hour         (0-23)
 └───────────── minute       (0-59)
```

## Reference

### `cronFromMap`

Convert a schedule **map** to a five-field cron string. Reads keys `minute`, `hour`, `dayOfMonth`, `month`, `dayOfWeek`; absent keys default to `*`. Errors if the input is not a map — use `cronFromAny` when the input may be a string.

Keywords: cron, schedule, map, convert, fields, expression

```yaml
# onReconcile Path B — input is guaranteed a map by the when: gate
- name: "{{ .metadata.name }}"
  schedule: "{{ cronFromMap .spec.schedule }}"
  when:
    - field: spec.schedule
      typeOf: map
```

```
{minute: "*/5", hour: "0", dayOfMonth: "*", month: "*", dayOfWeek: "1"}
→ "*/5 0 * * 1"
```

---

### `cronFromAny`

Convert a schedule value to a five-field cron string. Accepts either shape:

- **map** — reads keys `minute`, `hour`, `dayOfMonth`, `month`, `dayOfWeek`; absent keys default to `*`
- **string** — treated as an already-complete cron expression and normalised
- **nil / unknown** — returns `"* * * * *"`

Keywords: cron, schedule, convert, string, normalize, any

```yaml
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
```

```
{minute: "*/5", hour: "0", dayOfMonth: "*", month: "*", dayOfWeek: "1"}
→ "*/5 0 * * 1"

"*/5 0 * * 1"
→ "*/5 0 * * 1"   (string passthrough, normalised)
```

`cronFromAny` replaces the old conditional pattern:

```yaml
# Old:
schedule: >
  {{ if typeMap .spec.schedule }}{{ cronFromMap .spec.schedule }}
  {{ else }}{{ cronNormalize .spec.schedule }}{{ end }}

# New:
schedule: "{{ cronFromAny .spec.schedule }}"
```

---

### `cronToMap`

Convert a cron string to the structured map shape. For use in **conversion path specs only** — the result is a nested map, not a plain string.

- **string** — splits into five named fields
- **map** — returned as-is
- `@`-macros expanded before splitting (`@hourly` → minute:`0`, hour:`*`, …)

Keywords: cron, schedule, parse, fields, map, convert, structured

```yaml
# v1 → v2 conversion path
- from: v1
  to: v2
  spec:
    schedule: "{{ cronToMap .spec.schedule }}"
```

```
"*/5 0 * * 1"
→ {minute:"*/5", hour:"0", dayOfMonth:"*", month:"*", dayOfWeek:"1"}

"@hourly"
→ {minute:"0", hour:"*", dayOfMonth:"*", month:"*", dayOfWeek:"*"}
```

`cronToMap` is only meaningful as a top-level conversion spec field value. It signals to the conversion engine to produce a nested object in the output rather than a string. Using it in `status.fields` or `onCreate` produces unexpected output.

---

### `cronNormalize`

Normalize a cron string: expand `@`-macros, trim whitespace, ensure exactly five fields. Returns `"* * * * *"` for empty or malformed input.

Keywords: cron, schedule, normalize, macro, expand, sanitize

```yaml
# value: "{{ cronNormalize .spec.schedule }}"
# "@daily"      → "0 0 * * *"
# "@hourly"     → "0 * * * *"
# "*/5 * * * *" → "*/5 * * * *"  (unchanged, already valid)
# ""            → "* * * * *"
```

---

### `cronDescribe`

Return a human-readable description of a cron expression. Useful in `status.fields` for the Console UI.

Keywords: cron, schedule, describe, human-readable, display, status

```yaml
- path: scheduleDescription
  value: "{{ cronDescribe .spec.schedule }}"
```

| Expression | Description |
|------------|-------------|
| `*/5 * * * *` | Every 5 minutes |
| `0 * * * *` | Every hour |
| `0 2 * * *` | At 2:0 every day |
| `0 2 * * 1` | At 02:00 on Mondays |

---

### `cronValid`

## cronValid

Returns `true` when the expression is a syntactically valid cron expression with all fields within their correct ranges.

```text
# value: "{{ cronValid .spec.schedule }}"
# "0 2 * * 1-5"  → true
# "25 * * * *"   → true  (minute 25 is valid)
# "0 25 * * *"   → false (hour 25 invalid)
# "0 2 32 * *"   → false (day 32 invalid)
```

Keywords: cron, schedule, validate, valid, check, boolean

```yaml
validation:
  rules:
    - field: "{{ cronValid .spec.schedule }}"
      equals: "true"
      message: "spec.schedule must be a valid cron expression"
      action: deny
```

---

### `cronExpr`

Build a five-field cron expression from five explicit string parts. Empty parts default to `*`.

Keywords: cron, schedule, build, expression, fields, construct

```yaml
# value: "{{ cronExpr .spec.schedule.minute .spec.schedule.hour .spec.schedule.dayOfMonth .spec.schedule.month .spec.schedule.dayOfWeek }}"
# minute="*/1", hour="*", dom="*", month="*", dow="*" → "*/1 * * * *"
```

Prefer `cronFromMap .spec.schedule` over `cronExpr` with five separate field navigations — it is shorter, handles both schedule shapes, and does not fail if the schedule is a flat string rather than a map.

---

### `cronMinute` / `cronHour` / `cronDom` / `cronMonth` / `cronDow`

Extract a single field by position from a cron string. Returns `*` for empty input. Returns an error for strings that don't expand to five fields.

```yaml
# value: "{{ cronMinute \"*/5 2 * * 1\" }}"   → "*/5"
# value: "{{ cronHour   \"*/5 2 * * 1\" }}"   → "2"
# value: "{{ cronDom    \"0 0 15 * *\" }}"    → "15"
# value: "{{ cronMonth  \"0 0 1 6 *\" }}"     → "6"
# value: "{{ cronDow    \"0 0 * * 1\" }}"     → "1"
```

These are useful when you need a single field in isolation. For splitting a full cron string into all five fields for storage as a structured map, use `cronToMap` instead.

---

### `cronField`

Extract a field by index (0–4). The general form of the five named extractors above.

```yaml
# value: "{{ cronField .spec.schedule 0 }}"   → minute field
# value: "{{ cronField .spec.schedule 3 }}"   → month field
```

---

## Supported `@`-macros

| Macro | Expands to | Meaning |
|-------|-----------|---------|
| `@yearly` / `@annually` | `0 0 1 1 *` | Once a year, Jan 1 at midnight |
| `@monthly` | `0 0 1 * *` | Once a month, 1st at midnight |
| `@weekly` | `0 0 * * 0` | Once a week, Sunday at midnight |
| `@daily` / `@midnight` | `0 0 * * *` | Once a day at midnight |
| `@hourly` | `0 * * * *` | Once an hour at minute 0 |

---

## Quick reference

| Note | Accepts | Returns | Use in |
|------|---------|---------|--------|
| `cronFromMap` | `map` only (errors on string) | `string` (cron expr) | onReconcile behind `typeOf: map` gate |
| `cronFromAny` | `map` or `string` | `string` (cron expr) | normalize, status, conversion, unknown-shape input |
| `cronToMap` | `string` (map returned as-is) | `map` (nested object) | conversion path spec only |
| `cronExpr` | five `string` parts | `string` | status, reconcile |
| `cronNormalize` | `string` | `string` | normalize, status |
| `cronDescribe` | `string` | `string` | status (human display) |
| `cronValid` | `string` | `bool` | validation rules |
| `cronMinute` / `cronHour` / `cronDom` / `cronMonth` / `cronDow` | `string` | `string` | single-field extraction |
| `cronField` | `string`, `int` | `string` | single-field extraction by index |

---

**Next →** [06 — Random Notes](06-random.md)
