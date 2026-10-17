# 11 — Quantity Notes

Quantity notes enable resource budget arithmetic in templates using Kubernetes-style CPU and memory quantities. These are the building blocks for multi-tenant resource allocation — "give each tenant 1/N of available capacity" — without writing Go.

## What these notes operate on

Kubernetes quantities use standard SI suffixes (m, K, M, G, T, P, E) and binary suffixes (Ki, Mi, Gi, Ti, Pi, Ei). CPU is measured in milli-cores (`m`), memory in bytes with binary suffixes.

```yaml
# CPU:    "100m" = 0.1 core, "500m" = 0.5 core, "2" = 2 cores
# Memory: "512Mi" = 536870912 bytes, "1Gi" = 1073741824 bytes
```

---

## Reference

### `parseQuantity`

Convert a Kubernetes quantity string to `float64`. Useful as input to math notes for arithmetic.

Keywords: quantity, resource, cpu, memory, parse, float, kubernetes, budget

```yaml
# value: "{{ parseQuantity \"100m\" }}"       → 0.1
# value: "{{ parseQuantity \"500m\" }}"       → 0.5
# value: "{{ parseQuantity \"2\" }}"          → 2.0
# value: "{{ parseQuantity \"1Gi\" }}"        → 1073741824.0
```

Returns an error when the string is not a valid Kubernetes quantity.

---

### `formatQuantity`

Convert a `float64` back to a canonical Kubernetes quantity string. Sub-unit CPU fractions are expressed in milli-cores.

Keywords: quantity, resource, cpu, memory, format, string, kubernetes, canonical

```yaml
# value: "{{ formatQuantity 0.1 }}"        → "100m"
# value: "{{ formatQuantity 0.5 }}"        → "500m"
# value: "{{ formatQuantity 1.0 }}"        → "1"
# value: "{{ formatQuantity 1073741824 }}" → "1Gi"
```

---

### `sumQuantity`

Add two Kubernetes quantity strings and return the canonical string sum. Both operands must be the same dimension (CPU or memory).

Keywords: quantity, resource, cpu, memory, add, arithmetic, sum, budget

```yaml
# value: "{{ sumQuantity \"100m\" \"200m\" }}"  → "300m"
# value: "{{ sumQuantity \"500m\" \"500m\" }}"  → "1"
# value: "{{ sumQuantity \"1Gi\" \"512Mi\" }}"  → "1536Mi"
```

---

### `subtractQuantity`

Subtract the second Kubernetes quantity from the first and return the canonical string representation of the difference. Both operands must be the same dimension (CPU or memory).

Keywords: quantity, resource, cpu, memory, subtract, arithmetic, difference, budget

```yaml
# value: "{{ subtractQuantity \"1Gi\" \"512Mi\" }}"  → "512Mi"
# value: "{{ subtractQuantity \"500m\" \"500m\" }}"  → "0"
# value: "{{ subtractQuantity \"100m\" \"200m\" }}"  → "-100m"
```

Use `parseQuantity` and `formatQuantity` to do arithmetic with math notes:

```yaml
# Divide CPU limit by number of tenants:
# value: "{{ formatQuantity (div (parseQuantity .spec.cpuLimit) 4) }}"
# → "250m" when cpuLimit is "1"

# Cap replica memory at node size × headroom:
# value: "{{ formatQuantity (mul (parseQuantity .spec.nodeMemory) 0.8) }}"

# Per-tenant quota from a pool:
# value: "{{ formatQuantity (div (parseQuantity .spec.poolCPU) (toFloat .spec.tenantCount)) }}"
```

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `parseQuantity` | `(q string)` | `(float64, error)` |
| `formatQuantity` | `(f float64)` | `(string, error)` |
| `sumQuantity` | `(a, b string)` | `(string, error)` |
| `subtractQuantity` | `(a, b string)` | `(string, error)` |

---

**Next →** [12 — Replica Notes](12-replica.md)
