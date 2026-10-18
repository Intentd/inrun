# Safe access

Read a nested field and fall back to a default when it is absent or of the wrong type, typed as a string, an integer or a boolean.

## Reference

| Function | Description |
|----------|-------------|
| [`getBoolOr`](#getboolor) | Returns `val` as `bool` if it is a `bool`, otherwise returns `def`. |
| [`getIntOr`](#getintor) | Returns `val` as `int` if it is a numeric type (`int`, `int64`, `float64`), otherwise returns `def`. |
| [`getOr`](#getor) | Returns `val` if non-empty, otherwise `def`. |
| [`getStringOr`](#getstringor) | Returns `val` as a string if it is a non-empty string, otherwise returns `def`. |

## Functions

### getBoolOr

`getBoolOr` returns `val` as `bool` if it is a `bool`, otherwise returns `def`.

```text
{{ getBoolOr .spec.debug false }}    → true
{{ getBoolOr .spec.suspend false }}  → false
```

### getIntOr

`getIntOr` returns `val` as `int` if it is a numeric type (`int`, `int64`, `float64`), otherwise returns `def`.

```text
{{ getIntOr .deployment.spec.replicas 1 }}  → 3
{{ getIntOr .spec.workers 1 }}             → 1
```

### getOr

`getOr` returns `val` if non-empty, otherwise `def`. The generic version — same semantics as `default` but reads more naturally when chaining with other functions.

```text
{{ getOr .spec.workers 4 }}  → 4
```

### getStringOr

`getStringOr` returns `val` as a string if it is a non-empty string, otherwise returns `def`.

```text
{{ getStringOr .spec.sidecarImage "envoyproxy/envoy:v1.30" }}  → "envoyproxy/envoy:v1.30"
```
