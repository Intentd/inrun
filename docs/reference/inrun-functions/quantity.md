# Quantity Notes

Quantity notes enable resource budget arithmetic in templates using Kubernetes-style CPU and memory quantities. These are the building blocks for multi-tenant resource allocation — "give each tenant 1/N of available capacity" — without writing Go.

## Reference

| Note | Description |
|------|-------------|
| `parseQuantity` | Convert a Kubernetes quantity string to `float64`. |
| `formatQuantity` | Convert a `float64` back to a canonical Kubernetes quantity string. |
| `sumQuantity` | Add two Kubernetes quantity strings and return the canonical string sum. |
| `subtractQuantity` | Subtract the second Kubernetes quantity from the first and return the canonical string representation of the difference. |

## Examples

```yaml
# parseQuantity
# value: "{{ parseQuantity \"100m\" }}"       → 0.1
# value: "{{ parseQuantity \"500m\" }}"       → 0.5
# value: "{{ parseQuantity \"2\" }}"          → 2.0
# value: "{{ parseQuantity \"1Gi\" }}"        → 1073741824.0

# formatQuantity
# value: "{{ formatQuantity 0.1 }}"        → "100m"
# value: "{{ formatQuantity 0.5 }}"        → "500m"
# value: "{{ formatQuantity 1.0 }}"        → "1"
# value: "{{ formatQuantity 1073741824 }}" → "1Gi"

# sumQuantity
# value: "{{ sumQuantity \"100m\" \"200m\" }}"  → "300m"
# value: "{{ sumQuantity \"500m\" \"500m\" }}"  → "1"
# value: "{{ sumQuantity \"1Gi\" \"512Mi\" }}"  → "1536Mi"

# subtractQuantity
# value: "{{ subtractQuantity \"1Gi\" \"512Mi\" }}"  → "512Mi"
# value: "{{ subtractQuantity \"500m\" \"500m\" }}"  → "0"
# value: "{{ subtractQuantity \"100m\" \"200m\" }}"  → "-100m"
# Divide CPU limit by number of tenants:
# value: "{{ formatQuantity (div (parseQuantity .spec.cpuLimit) 4) }}"
# → "250m" when cpuLimit is "1"

# Cap replica memory at node size × headroom:
# value: "{{ formatQuantity (mul (parseQuantity .spec.nodeMemory) 0.8) }}"

# Per-tenant quota from a pool:
# value: "{{ formatQuantity (div (parseQuantity .spec.poolCPU) (toFloat .spec.tenantCount)) }}"
```
