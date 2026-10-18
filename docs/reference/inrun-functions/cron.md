# Cron

Work with five-field cron expressions (minute hour day-of-month month day-of-week): read single fields, build and normalise expressions, convert between the string form and a map with one key per field, describe a schedule in words, and check that an expression is valid.

## Reference

| Function | Description |
|----------|-------------|
| [`cronDescribe`](#crondescribe) | Describes a cron expression in words, for display: "Every 5 minutes", "At 02:00 on Monday through Friday". |
| [`cronDom`](#crondom) | Returns the day-of-month field of a cron expression; "*" for an empty expression, and an error when it does not have five fields. |
| [`cronDow`](#crondow) | Returns the day-of-week field of a cron expression; "*" for an empty expression, and an error when it does not have five fields. |
| [`cronExpr`](#cronexpr) | Builds a five-field cron expression from five explicit string parts. |
| [`cronField`](#cronfield) | Extracts a field by index (0–4). |
| [`cronFromAny`](#cronfromany) | Converts a schedule value to a five-field cron string. |
| [`cronFromMap`](#cronfrommap) | Converts a schedule **map** to a five-field cron string. |
| [`cronHour`](#cronhour) | Returns the hour field of a cron expression; "*" for an empty expression, and an error when it does not have five fields. |
| [`cronMinute`](#cronminute) | Returns the minute field of a cron expression; "*" for an empty expression, and an error when it does not have five fields. |
| [`cronMonth`](#cronmonth) | Returns the month field of a cron expression; "*" for an empty expression, and an error when it does not have five fields. |
| [`cronNormalize`](#cronnormalize) | Normalizes a cron string: expand `@`-macros, trim whitespace, ensure exactly five fields. |
| [`cronToMap`](#crontomap) | Converts a cron string to the structured map shape. |
| [`cronValid`](#cronvalid) | Reports whether expr is a valid five-field cron expression, with every field in range. |

## Functions

### cronDescribe

`cronDescribe` describes a cron expression in words, for display: "Every 5 minutes", "At 02:00 on Monday through Friday". Expressions it cannot put into words come back as "Cron: &lt;expression&gt;"; "Invalid cron expression" when expr is not a five-field expression or a macro.

```text
{{ cronDescribe "0 2 * * 1-5" }}  → "At 02:00 on Monday through Friday"
{{ cronDescribe "*/15 * * * *" }}  → "Every 15 minutes"
```

See also: [`cronValid`](#cronvalid), [`nextCron`](../time/#nextcron)

### cronDom

`cronDom` returns the day-of-month field of a cron expression; "*" for an empty expression, and an error when it does not have five fields.

```text
{{ cronDom "0 0 15 * *" }}  → "15"
```

### cronDow

`cronDow` returns the day-of-week field of a cron expression; "*" for an empty expression, and an error when it does not have five fields.

```text
{{ cronDow "0 0 * * 1" }}  → "1"
```

### cronExpr

`cronExpr` builds a five-field cron expression from five explicit string parts. Empty parts default to `*`.

```text
{{ cronExpr "*/15" "" "" "" "1-5" }}  → "*/15 * * * 1-5"
```

### cronField

`cronField` extracts a field by index (0–4). The general form of the five named extractors above.

```text
{{ cronField "*/5 2 * * 1" 1 }}  → "2"
```

### cronFromAny

`cronFromAny` converts a schedule value to a five-field cron string. Accepts either shape:

```text
{{ cronFromAny .spec.schedule }}  → "*/5 0 * * 1"
{{ cronFromAny "0 2 * * *" }}    → "0 2 * * *"
```

### cronFromMap

`cronFromMap` converts a schedule **map** to a five-field cron string. Reads keys `minute`, `hour`, `dayOfMonth`, `month`, `dayOfWeek`; absent keys default to `*`. Errors if the input is not a map — use `cronFromAny` when the input may be a string.

```text
{{ cronFromMap .spec.schedule }}  → "*/5 0 * * 1"
```

### cronHour

`cronHour` returns the hour field of a cron expression; "*" for an empty expression, and an error when it does not have five fields.

```text
{{ cronHour "*/5 2 * * 1" }}  → "2"
```

### cronMinute

`cronMinute` returns the minute field of a cron expression; "*" for an empty expression, and an error when it does not have five fields.

```text
{{ cronMinute "*/5 2 * * 1" }}  → "*/5"
```

### cronMonth

`cronMonth` returns the month field of a cron expression; "*" for an empty expression, and an error when it does not have five fields.

```text
{{ cronMonth "0 0 1 6 *" }}  → "6"
```

### cronNormalize

`cronNormalize` normalizes a cron string: expand `@`-macros, trim whitespace, ensure exactly five fields. Returns `"* * * * *"` for empty or malformed input.

```text
{{ cronNormalize "@daily" }}  → "0 0 * * *"
```

### cronToMap

`cronToMap` converts a cron string to the structured map shape. For use in **conversion path specs only** — the result is a nested map, not a plain string.

```text
{{ (cronToMap "*/5 0 * * 1").minute }}  → "*/5"
```

### cronValid

`cronValid` reports whether expr is a valid five-field cron expression, with every field in range. Macros (@daily), steps, ranges and lists are accepted.

```text
{{ cronValid "0 2 * * 1-5" }}  → true
{{ cronValid "0 25 * * *" }}   → false
```
