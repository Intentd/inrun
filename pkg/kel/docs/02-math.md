# 02 — Math Notes

Math notes perform arithmetic on numeric values from the CR spec. All math notes accept `int`, `int64`, `float64`, or numeric strings — they handle the mixed types that come out of Kubernetes unstructured objects without requiring explicit conversions.

## Number types in unstructured CRs

Kubernetes stores numbers in JSON. When deserialized into `map[string]interface{}`, JSON numbers come back as `float64`. Integers that fit in int64 are returned as `int64` after normalization. All math notes accept both:

```yaml
spec:
  replicas: 3        # arrives as float64(3) from JSON
  basePort: 8080     # arrives as float64(8080)
```

## Reference

### `add`

Add two numbers. Returns `int64` when the result is whole, `float64` otherwise.

Keywords: math, arithmetic, add, sum, number, plus

```yaml
# value: "{{ add .spec.basePort 1000 }}"
# basePort=8080 → 9080
```

---

### `sub`

Subtract the second number from the first.

Keywords: math, arithmetic, subtract, difference, minus, number

```yaml
# value: "{{ sub .spec.replicas 1 }}"
# replicas=3 → 2
```

---

### `mul`

Multiply two numbers.

Keywords: math, arithmetic, multiply, product, times, number

```yaml
# value: "{{ mul .spec.replicas 2 }}"
# replicas=3 → 6
```

---

### `div`

Divide the first number by the second. Returns an error on division by zero.

Keywords: math, arithmetic, divide, quotient, number, ratio

```yaml
# value: "{{ div .spec.totalWorkers 4 }}"
# totalWorkers=8 → 2
```

---

### `mod`

Integer modulo. Returns `int64`.

Keywords: math, arithmetic, modulo, remainder, integer, modulus

```yaml
# value: "{{ mod .spec.port 100 }}"
# port=8080 → 80
```

---

### `min`

Return the smaller of two numbers. Use to cap a value at an upper bound.

Keywords: math, minimum, clamp, cap, bound, number

```yaml
# Cap replicas at 10
# value: "{{ min .spec.replicas 10 }}"
# replicas=15 → 10
# replicas=3  → 3
```

---

### `max`

Return the larger of two numbers. Use to enforce a minimum floor.

Keywords: math, maximum, clamp, floor, bound, number

```yaml
# Ensure at least 2 replicas
# value: "{{ max .spec.replicas 2 }}"
# replicas=1 → 2
# replicas=5 → 5
```

---

### `clamp`

Constrain a value to the range `[lo, hi]`. Equivalent to `max(lo, min(hi, val))`.

Keywords: math, clamp, range, bounds, constrain, number, limit

```yaml
# value: "{{ clamp .spec.replicas 1 20 }}"
# replicas=0  → 1
# replicas=5  → 5
# replicas=25 → 20
```

Useful in mutation rules to enforce business limits without a deny action.

---

### `abs`

Return the absolute value of a number.

Keywords: math, absolute, positive, number

```yaml
# value: "{{ abs .spec.offsetSeconds }}"
# -300 → 300
#  300 → 300
```

---

## Combining math notes

Math notes compose naturally:

```yaml
# Total memory limit = replicas × per-pod limit, capped at cluster max
# value: "{{ clamp (mul .spec.replicas .spec.memoryPerPod) 0 65536 }}"
```

```yaml
# Port = base + index (forEach: as index)
# value: "{{ add .spec.basePort .index }}"
```

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `add` | `(a, b any)` | `int64` or `float64` |
| `sub` | `(a, b any)` | `int64` or `float64` |
| `mul` | `(a, b any)` | `int64` or `float64` |
| `div` | `(a, b any)` | `int64` or `float64` |
| `mod` | `(a, b any)` | `int64` |
| `min` | `(a, b any)` | `int64` or `float64` |
| `max` | `(a, b any)` | `int64` or `float64` |
| `clamp` | `(val, lo, hi any)` | `int64` or `float64` |
| `abs` | `(a any)` | `int64` or `float64` |

All math notes return an error on non-numeric input. Template execution halts and the error surfaces in the CR's `Ready` condition.

---

**Next →** [03 — Conditional Notes](03-conditional.md)
