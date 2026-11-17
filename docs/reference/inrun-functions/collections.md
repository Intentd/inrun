# Collections

Convert a value that may arrive in several shapes (a native list or map, or the same as a JSON or YAML string) into a list, a map or a string, so the functions after it see one shape.

Build lists and maps, and read them without guarding every step: make a list or a map, get an element by index or a value by key, test membership, take keys and values, or keep and drop keys. Out-of-range and missing keys give zero values, not errors.

## Reference

| Function | Description |
|----------|-------------|
| [`asList`](#aslist) | Converts input to `[]interface{}`. |
| [`asMap`](#asmap) | Converts input to `map[string]interface{}`. |
| [`asString`](#asstring) | Converts any value to a string. |
| [`list`](#list) | Returns its arguments as a list, for functions that take several values in one argument, such as the failed values of expectState. |
| [`listGet`](#listget) | Returns the element at a given index. |
| [`listHas`](#listhas) | Reports whether a list contains a value. |
| [`listLen`](#listlen) | Returns the number of elements in a list. |
| [`map`](#map) | Returns a map from alternating keys and values, the counterpart of list. |
| [`mapGet`](#mapget) | Returns a map value by key. |
| [`mapKeys`](#mapkeys) | Returns all keys of a map as sorted `[]string`. |
| [`mapOmit`](#mapomit) | Returns a new map with the specified keys removed. |
| [`mapPick`](#mappick) | Returns a new map containing only the specified keys. |
| [`mapValues`](#mapvalues) | Returns all values of a map as `[]interface{}`. |

## Functions

### asList

`asList` converts input to `[]interface{}`. Accepts native slice, YAML list string, or JSON array string.

```text
{{ asList "[\"us-east-1\",\"eu-west-1\"]" }}  → [us-east-1 eu-west-1]
{{ len (asList .spec.regions) }}  → 2
```

### asMap

`asMap` converts input to `map[string]interface{}`. Accepts native map, YAML map string, or JSON object string.

```text
{{ (asMap "{\"tier\":\"web\"}").tier }}  → "web"
```

### asString

`asString` converts any value to a string. For maps and slices this produces a JSON encoding.

```text
{{ asString 3 }}                   → "3"
{{ asString .deployment.spec.replicas }}  → "3"
```

### list

`list` returns its arguments as a list, for functions that take several values in one argument, such as the failed values of expectState. With no arguments it returns an empty list.

```text
{{ list "Failed" "Unknown" }}  → [Failed Unknown]
{{ listLen (list 1 2 3) }}  → 3
```

### listGet

`listGet` returns the element at a given index. Returns `nil` for out-of-range indices — no panic.

```text
{{ listGet .spec.regions 0 }}  → "us-east-1"
{{ listGet .spec.regions 9 }}  → <no value>
```

### listHas

`listHas` reports whether a list contains a value. Comparison is by `==`.

```text
{{ listHas .spec.regions "eu-west-1" }}  → true
```

### listLen

`listLen` returns the number of elements in a list. Returns `0` for non-lists.

```text
{{ listLen .spec.regions }}  → 2
```

### map

`map` returns a map from alternating keys and values, the counterpart of list. Keys that are not strings are written as text. With no arguments it returns an empty map. An odd number of arguments is an error: a key without a value.

```text
{{ mapGet (map "region" "eu-west-1" "replicas" 3) "replicas" }}  → 3
{{ mapKeys (map "b" 2 "a" 1) }}  → [a b]
```

### mapGet

`mapGet` returns a map value by key. Returns `nil` when the map is absent or the key is missing.

```text
{{ mapGet .deployment.metadata.labels "app" }}      → "web"
{{ mapGet .deployment.metadata.labels "missing" }}  → <no value>
```

### mapKeys

`mapKeys` returns all keys of a map as sorted `[]string`. Returns an empty slice for non-maps.

```text
{{ mapKeys .spec.selector }}  → [app tier]
```

### mapOmit

`mapOmit` returns a new map with the specified keys removed.

```text
{{ mapOmit .metadata.labels "internal-key" "debug" }}
```

### mapPick

`mapPick` returns a new map containing only the specified keys. Missing keys are silently omitted.

```text
{{ mapPick .spec "image" "replicas" }}
```

### mapValues

`mapValues` returns all values of a map as `[]interface{}`. Returns an empty slice for non-maps.

```text
{{ len (mapValues .spec.selector) }}  → 2
```
