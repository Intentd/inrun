# Safe Access Notes

Safe access notes return a typed value with a fallback default. They remove the need to combine `typeOf` + `default` + type conversion for the common pattern of "get this field or use a default if it's absent or the wrong type".

## Reference

| Note | Description |
|------|-------------|
| `getOr` | Return `val` if non-empty, otherwise `def`. |
| `getStringOr` | Return `val` as a string if it is a non-empty string, otherwise return `def`. |
| `getIntOr` | Return `val` as `int` if it is a numeric type (`int`, `int64`, `float64`), otherwise return `def`. |
| `getBoolOr` | Return `val` as `bool` if it is a `bool`, otherwise return `def`. |

## Examples

```yaml
# getOr
# value: "{{ getOr .spec.replicas 1 }}"
# replicas=3       → 3
# replicas absent  → 1

# getStringOr
# value: "{{ getStringOr .spec.image \"nginx:latest\" }}"
# spec.image="busybox:1.35" → "busybox:1.35"
# spec.image absent         → "nginx:latest"
# spec.image=3 (number)     → "nginx:latest"  (not a string — use def)

# getIntOr
# value: "{{ getIntOr .spec.replicas 1 }}"
# spec.replicas=3      → 3
# spec.replicas=3.0    → 3  (float64 → int)
# spec.replicas absent → 1
# spec.replicas="3"    → 1  (string not accepted — use def)

# getBoolOr
# value: "{{ getBoolOr .spec.enabled false }}"
# spec.enabled=true   → true
# spec.enabled=false  → false
# spec.enabled absent → false
# spec.enabled="true" → false  (string not accepted — use boolDefault)
# Clamp replicas to [1, 20] with a default of 2
# value: "{{ clamp (getIntOr .spec.replicas 2) 1 20 }}"

# Use replicas or a field-based default
# value: "{{ getIntOr .spec.replicas (getIntOr .spec.defaultReplicas 1) }}"
```
