# Types

Inspect and convert types: what type a value has (map, slice, string, number, bool, null), whether it is empty, and conversion to strings, numbers, booleans, lists and JSON.

## Reference

| Function | Description |
|----------|-------------|
| [`isComposite`](#iscomposite) | Reports whether v holds other values: a map or a slice. |
| [`isEmpty`](#isempty) | Returns `true` when the value is nil, an empty string, an empty slice, or an empty map. |
| [`isScalar`](#isscalar) | Reports whether v is a single value: a string, number or bool. |
| [`len`](#len) | Returns the length of a string, slice, or map. |
| [`toBool`](#tobool) | Converts a value to `bool`. |
| [`toFloat`](#tofloat) | Converts any value to `float64`. |
| [`toInt`](#toint) | Converts any value to `int64`. |
| [`toJSON`](#tojson) | Marshals any value to a JSON string. |
| [`toList`](#tolist) | Converts a comma-separated string to a list of trimmed strings. |
| [`toString`](#tostring) | Converts any value to its string representation. |
| [`typeBool`](#typebool) | Reports whether v is a boolean; the same as eq (typeOf v) "bool". |
| [`typeList`](#typelist) | Reports whether v is a list (a YAML array); the same as eq (typeOf v) "slice". |
| [`typeMap`](#typemap) | Reports whether v is a map (a YAML object); the same as eq (typeOf v) "map". |
| [`typeNull`](#typenull) | Reports whether v is null or absent; the same as eq (typeOf v) "null". |
| [`typeNumber`](#typenumber) | Reports whether v is a number; the same as eq (typeOf v) "number". |
| [`typeOf`](#typeof) | Returns the runtime type name of any value. |
| [`typeString`](#typestring) | Reports whether v is a string; the same as eq (typeOf v) "string". |

## Functions

### isComposite

`isComposite` reports whether v holds other values: a map or a slice.

```text
{{ isComposite .spec.selector }}  → true
{{ isComposite .spec.replicas }}  → false
```

### isEmpty

`isEmpty` returns `true` when the value is nil, an empty string, an empty slice, or an empty map.

```text
{{ isEmpty .spec.logLevel }}  → true
{{ isEmpty .spec.regions }}   → false
```

### isScalar

`isScalar` reports whether v is a single value: a string, number or bool.

```text
{{ isScalar .deployment.spec.replicas }}  → true
{{ isScalar .spec.selector }}  → false
```

### len

`len` returns the length of a string, slice, or map. Overrides Go's built-in `len` with one that handles all three types uniformly.

```text
{{ len .spec.regions }}            → 2
{{ len .deployment.metadata.name }}  → 3
```

### toBool

`toBool` converts a value to `bool`. Returns an error for unrecognized strings.

```text
{{ toBool "yes" }}  → true
```

### toFloat

`toFloat` converts any value to `float64`.

```text
{{ toFloat "0.75" }}  → 0.75
```

### toInt

`toInt` converts any value to `int64`. Truncates floats.

```text
{{ toInt "3" }}  → 3
{{ toInt 3.7 }}  → 3
```

### toJSON

`toJSON` marshals any value to a JSON string. Useful for embedding structured data in annotations, ConfigMap values, or status fields.

```text
{{ toJSON .spec.selector }}  → {"app":"web","tier":"frontend"}
```

### toList

`toList` converts a comma-separated string to a list of trimmed strings. Returns an empty list for empty strings. Use it where a comma-separated value, such as an annotation, needs to be treated as a list.

```text
{{ toList "a, b, c" }}  → [a b c]
```

### toString

`toString` converts any value to its string representation. Uses `fmt.Sprintf("%v", v)`.

```text
{{ toString .deployment.spec.replicas }}  → "3"
```

### typeBool

`typeBool` reports whether v is a boolean; the same as eq (typeOf v) "bool".

```text
{{ if typeBool .spec.schedule }}…{{ end }}
```

### typeList

`typeList` reports whether v is a list (a YAML array); the same as eq (typeOf v) "slice".

```text
{{ if typeList .spec.schedule }}…{{ end }}
```

### typeMap

`typeMap` reports whether v is a map (a YAML object); the same as eq (typeOf v) "map".

```text
{{ if typeMap .spec.schedule }}…{{ end }}
```

### typeNull

`typeNull` reports whether v is null or absent; the same as eq (typeOf v) "null".

```text
{{ if typeNull .spec.schedule }}…{{ end }}
```

### typeNumber

`typeNumber` reports whether v is a number; the same as eq (typeOf v) "number".

```text
{{ if typeNumber .spec.schedule }}…{{ end }}
```

### typeOf

`typeOf` returns the runtime type name of any value. The canonical way to branch on field type.

```text
{{ typeOf .spec.schedule }}  → "map"
{{ typeOf .spec.regions }}   → "slice"
```

### typeString

`typeString` reports whether v is a string; the same as eq (typeOf v) "string".

```text
{{ if typeString .spec.schedule }}…{{ end }}
```
