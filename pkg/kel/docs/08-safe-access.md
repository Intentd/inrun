# 08 — Safe Access Notes

Safe access notes return a typed value with a fallback default. They remove the need to combine `typeOf` + `default` + type conversion for the common pattern of "get this field or use a default if it's absent or the wrong type".

## When to use safe access vs `default`

`default` (from [conditional notes](03-conditional.md)) is generic — it works with any type but returns `any`. Safe access notes return a typed value, which matters when the result is passed to a function that expects a specific type.

```yaml
# default — returns any, fine for string fields
# value: "{{ default .spec.logLevel \"info\" }}"

# getStringOr — same result but explicitly typed
# value: "{{ getStringOr .spec.logLevel \"info\" }}"

# getIntOr — necessary when passing to math notes
# value: "{{ add (getIntOr .spec.replicas 1) 2 }}"
```

## Reference

### `getOr`

Return `val` if non-empty, otherwise `def`. The generic version — same semantics as `default` but reads more naturally when chaining with other notes.

Keywords: safe, default, fallback, access, nil, absent, get

```yaml
# value: "{{ getOr .spec.replicas 1 }}"
# replicas=3       → 3
# replicas absent  → 1
```

---

### `getStringOr`

Return `val` as a string if it is a non-empty string, otherwise return `def`.

Keywords: safe, default, fallback, string, typed, absent, get

```yaml
# value: "{{ getStringOr .spec.image \"nginx:latest\" }}"
# spec.image="busybox:1.35" → "busybox:1.35"
# spec.image absent         → "nginx:latest"
# spec.image=3 (number)     → "nginx:latest"  (not a string — use def)
```

Unlike `default`, `getStringOr` rejects non-string types — it does not coerce.

---

### `getIntOr`

Return `val` as `int` if it is a numeric type (`int`, `int64`, `float64`), otherwise return `def`.

Keywords: safe, default, fallback, integer, typed, absent, get, number

```yaml
# value: "{{ getIntOr .spec.replicas 1 }}"
# spec.replicas=3      → 3
# spec.replicas=3.0    → 3  (float64 → int)
# spec.replicas absent → 1
# spec.replicas="3"    → 1  (string not accepted — use def)
```

For numeric fields from Kubernetes unstructured objects (which arrive as `float64`), `getIntOr` is the cleanest way to extract a safe integer.

---

### `getBoolOr`

Return `val` as `bool` if it is a `bool`, otherwise return `def`.

Keywords: safe, default, fallback, boolean, typed, absent, get, bool

```yaml
# value: "{{ getBoolOr .spec.enabled false }}"
# spec.enabled=true   → true
# spec.enabled=false  → false
# spec.enabled absent → false
# spec.enabled="true" → false  (string not accepted — use boolDefault)
```

For strict boolean fields: `getBoolOr` only accepts native `bool`. For string representations of booleans, use `toBool` (from [type notes](04-types.md)) or `boolDefault` (from [conditional notes](03-conditional.md)).

---

## Composing with other notes

Safe access notes compose cleanly with math and conditional notes:

```yaml
# Clamp replicas to [1, 20] with a default of 2
# value: "{{ clamp (getIntOr .spec.replicas 2) 1 20 }}"

# Use replicas or a field-based default
# value: "{{ getIntOr .spec.replicas (getIntOr .spec.defaultReplicas 1) }}"
```

---

## Quick reference

| Note | Signature | Returns | Accepts |
|------|-----------|---------|---------|
| `getOr` | `(val, def any)` | `any` | anything non-empty |
| `getStringOr` | `(val any, def string)` | `string` | `string` only |
| `getIntOr` | `(val any, def int)` | `int` | `int`, `int64`, `float64` |
| `getBoolOr` | `(val any, def bool)` | `bool` | `bool` only |

---

**Next →** [09 — Kubernetes Notes](09-kubernetes.md)
