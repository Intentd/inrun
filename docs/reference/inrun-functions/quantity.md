# Quantity

Work with Kubernetes resource quantities ("500m", "2Gi"): parse, compare, add and subtract them, keeping the canonical form.

## Reference

| Function | Description |
|----------|-------------|
| [`formatQuantity`](#formatquantity) | Converts a `float64` back to a canonical Kubernetes quantity string. |
| [`minusQuantity`](#minusquantity) | Subtracts the second Kubernetes quantity from the first and returns the canonical string representation of the difference. |
| [`parseQuantity`](#parsequantity) | Converts a Kubernetes quantity string to `float64`. |
| [`sumQuantity`](#sumquantity) | Adds two Kubernetes quantity strings and returns the canonical string sum. |

## Functions

### formatQuantity

`formatQuantity` converts a `float64` back to a canonical Kubernetes quantity string. Sub-unit CPU fractions are expressed in milli-cores.

```text
{{ formatQuantity 0.5 }}         → "500m"
{{ formatQuantity 1073741824 }}  → "1Gi"
```

### minusQuantity

`minusQuantity` subtracts the second Kubernetes quantity from the first and returns the canonical string representation of the difference. Both operands must be the same dimension (CPU or memory).

```text
{{ minusQuantity "1Gi" "512Mi" }}  → "512Mi"
```

### parseQuantity

`parseQuantity` converts a Kubernetes quantity string to `float64`. Useful as input to math functions for arithmetic.

```text
{{ parseQuantity "500m" }}  → 0.5
{{ parseQuantity "1Gi" }}   → 1.073741824e+09
```

### sumQuantity

`sumQuantity` adds two Kubernetes quantity strings and returns the canonical string sum. Both operands must be the same dimension (CPU or memory).

```text
{{ sumQuantity "100m" "200m" }}  → "300m"
{{ sumQuantity "1Gi" "512Mi" }}  → "1536Mi"
```
