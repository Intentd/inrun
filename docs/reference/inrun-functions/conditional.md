# Conditional

Choose between values inside an expression: defaults for empty values, the first non-empty of several, and ternaries, so a template does not need an if block for a one-line choice.

## Reference

| Function | Description |
|----------|-------------|
| [`boolDefault`](#booldefault) | Returns the field's boolean value if it is a `bool`, otherwise returns `def`. |
| [`boolTernary`](#boolternary) | Is like `ternary` but requires a strict `bool` argument. |
| [`coalesce`](#coalesce) | Returns the first non-empty value from a variadic list. |
| [`default`](#default) | Returns `val` if non-empty, otherwise returns `def`. |
| [`empty`](#empty) | Returns `true` when the value is empty (nil, `""`, `0`, `false`, empty slice, empty map, or `"<no value>"`). |
| [`eqTernary`](#eqternary) | Returns `trueVal` when `val` equals `target` (string comparison), `falseVal` otherwise. |
| [`notEmpty`](#notempty) | Returns `true` when the value is non-empty. |
| [`ternary`](#ternary) | Returns `trueVal` when `condition` is truthy, `falseVal` otherwise. |

## Functions

### boolDefault

`boolDefault` returns the field's boolean value if it is a `bool`, otherwise returns `def`. Useful when a boolean field may be absent (nil).

```text
{{ boolDefault .spec.suspend false }}  → false
```

### boolTernary

`boolTernary` is like `ternary` but requires a strict `bool` argument. Use this when the field is declared as a boolean in the CRD schema — it avoids the truthiness ambiguity of `ternary` for values like `0`, `""`, or `[]`.

```text
{{ boolTernary .spec.debug "debug" "info" }}  → "debug"
```

### coalesce

`coalesce` returns the first non-empty value from a variadic list. Useful when a field can come from multiple sources with a final fallback.

```text
{{ coalesce .spec.sidecarImage .spec.defaultSidecarImage "envoyproxy/envoy:v1.30" }}  → "envoyproxy/envoy:v1.30"
```

### default

`default` returns `val` if non-empty, otherwise returns `def`. "Empty" means nil, `""`, `0`, `false`, empty slice, or empty map.

```text
{{ default "info" .spec.logLevel }}  → "info"
{{ .spec.logLevel | default "info" }}  → "info"
```

### empty

`empty` returns `true` when the value is empty (nil, `""`, `0`, `false`, empty slice, empty map, or `"<no value>"`).

```text
{{ empty .spec.logLevel }}  → true
```

### eqTernary

`eqTernary` returns `trueVal` when `val` equals `target` (string comparison), `falseVal` otherwise. Shorthand for the `boolTernary (eq val target) trueVal falseVal` pattern — useful when branching on a string field value such as a status string, a mode flag, or a cross-CRD `found` result.

```text
{{ eqTernary .spec.mode "production" "strict" "permissive" }}  → "strict"
```

### notEmpty

`notEmpty` returns `true` when the value is non-empty. The inverse of `empty`.

```text
{{ notEmpty .spec.mode }}  → true
```

### ternary

`ternary` returns `trueVal` when `condition` is truthy, `falseVal` otherwise. Condition truthiness follows Go rules: non-empty string, non-zero number, non-empty slice/map, `true` bool.

```text
{{ ternary .spec.debug "debug" "info" }}  → "debug"
```
