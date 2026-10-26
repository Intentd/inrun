# Math

Arithmetic on values that may arrive as integers, floats or numeric strings: add, subtract, multiply, divide, modulo, absolute value, minimum, maximum and clamping. Division and modulo by zero are errors.

## Reference

| Function | Description |
|----------|-------------|
| [`abs`](#abs) | Returns the absolute value of a number. |
| [`add`](#add) | Adds two numbers. |
| [`clamp`](#clamp) | Constrains a value to the range `[lo, hi]`. |
| [`div`](#div) | Divides the first number by the second. |
| [`max`](#max) | Returns the larger of two numbers. |
| [`min`](#min) | Returns the smaller of two numbers. |
| [`mod`](#mod) | Returns the remainder of dividing the first number by the second, as an integer (`int64`). |
| [`mul`](#mul) | Multiplies two numbers. |
| [`sub`](#sub) | Subtracts the second number from the first. |

## Functions

### abs

`abs` returns the absolute value of a number.

```text
{{ abs -300 }}  → 300
```

### add

`add` adds two numbers. Returns `int64` when the result is whole, `float64` otherwise.

```text
{{ add 8080 1000 }}  → 9080
```

### clamp

`clamp` constrains a value to the range `[lo, hi]`. Equivalent to `max(lo, min(hi, val))`.

```text
{{ clamp 25 1 20 }}  → 20
{{ clamp 0 1 20 }}   → 1
```

### div

`div` divides the first number by the second. Returns an error on division by zero.

```text
{{ div 8 4 }}  → 2
```

### max

`max` returns the larger of two numbers. Use to enforce a minimum floor.

```text
{{ max .deployment.spec.replicas 5 }}  → 5
```

### min

`min` returns the smaller of two numbers. Use to cap a value at an upper bound.

```text
{{ min .deployment.spec.replicas 10 }}  → 3
```

### mod

`mod` returns the remainder of dividing the first number by the second, as an integer (`int64`). An error when the second is zero.

```text
{{ mod 8080 100 }}  → 80
```

### mul

`mul` multiplies two numbers.

```text
{{ mul .deployment.spec.replicas 2 }}  → 6
```

### sub

`sub` subtracts the second number from the first.

```text
{{ sub .deployment.spec.replicas 1 }}  → 2
```
