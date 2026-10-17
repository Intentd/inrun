# Math Notes

Math notes perform arithmetic on numeric values from the CR spec. All math notes accept `int`, `int64`, `float64`, or numeric strings — they handle the mixed types that come out of Kubernetes unstructured objects without requiring explicit conversions.

## Reference

| Note | Description |
|------|-------------|
| `add` | Add two numbers. |
| `sub` | Subtract the second number from the first. |
| `mul` | Multiply two numbers. |
| `div` | Divide the first number by the second. |
| `mod` | Integer modulo. |
| `min` | Return the smaller of two numbers. |
| `max` | Return the larger of two numbers. |
| `clamp` | Constrain a value to the range `[lo, hi]`. |
| `abs` | Return the absolute value of a number. |

## Examples

```yaml
# add
# value: "{{ add .spec.basePort 1000 }}"
# basePort=8080 → 9080

# sub
# value: "{{ sub .spec.replicas 1 }}"
# replicas=3 → 2

# mul
# value: "{{ mul .spec.replicas 2 }}"
# replicas=3 → 6

# div
# value: "{{ div .spec.totalWorkers 4 }}"
# totalWorkers=8 → 2

# mod
# value: "{{ mod .spec.port 100 }}"
# port=8080 → 80

# min
# Cap replicas at 10
# value: "{{ min .spec.replicas 10 }}"
# replicas=15 → 10
# replicas=3  → 3

# max
# Ensure at least 2 replicas
# value: "{{ max .spec.replicas 2 }}"
# replicas=1 → 2
# replicas=5 → 5

# clamp
# value: "{{ clamp .spec.replicas 1 20 }}"
# replicas=0  → 1
# replicas=5  → 5
# replicas=25 → 20

# abs
# value: "{{ abs .spec.offsetSeconds }}"
# -300 → 300
#  300 → 300
# Total memory limit = replicas × per-pod limit, capped at cluster max
# value: "{{ clamp (mul .spec.replicas .spec.memoryPerPod) 0 65536 }}"
# Port = base + index (forEach: as index)
# value: "{{ add .spec.basePort .index }}"
```
