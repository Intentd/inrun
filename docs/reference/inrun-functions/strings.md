# Strings

The string operations expressions need most: case, trimming, prefixes and suffixes, splitting and joining, concatenation, case conversion between camelCase and kebab-case, and shortening text for display.

## Reference

| Function | Description |
|----------|-------------|
| [`camelToKebab`](#cameltokebab) | Converts CamelCase or PascalCase to kebab-case. |
| [`concat`](#concat) | Joins any number of strings together with no separator. |
| [`contains`](#contains) | Reports whether substr is within s. |
| [`ellipsis`](#ellipsis) | Shortens s to at most n characters, ending in "..." when it was cut. |
| [`hasPrefix`](#hasprefix) | Reports whether s starts with prefix. |
| [`hasSuffix`](#hassuffix) | Reports whether s ends with suffix. |
| [`join`](#join) | Joins the elements of a list with sep between them. |
| [`repeat`](#repeat) | Returns s repeated count times. |
| [`replace`](#replace) | Returns s with every occurrence of old replaced by new. |
| [`split`](#split) | Splits a string by a separator into a slice. |
| [`toLower`](#tolower) | Returns s in lower case. |
| [`toUpper`](#toupper) | Returns s in upper case. |
| [`trim`](#trim) | Returns s without the leading and trailing characters in cutset. |
| [`trimPrefix`](#trimprefix) | Returns s without prefix, if it starts with it. |
| [`trimSpace`](#trimspace) | Returns s without leading and trailing white space. |
| [`trimSuffix`](#trimsuffix) | Returns s without suffix, if it ends with it. |

## Functions

### camelToKebab

`camelToKebab` converts CamelCase or PascalCase to kebab-case. Useful when deriving Kubernetes resource names from Go type names.

```text
{{ camelToKebab "WebsiteOperator" }}  → "website-operator"
```

### concat

`concat` joins any number of strings together with no separator. Useful for building domain names, resource name prefixes, or any value assembled from multiple parts.

```text
{{ concat "*." .spec.domain }}  → "*.api.example.com"
```

### contains

`contains` reports whether substr is within s.

```text
{{ contains .spec.image ":latest" }}  → true
```

### ellipsis

`ellipsis` shortens s to at most n characters, ending in "..." when it was cut. For text people read (messages, descriptions); not for names or label values, which may not end in a dot: use truncate there.

```text
{{ ellipsis .status.message 80 }}
```

### hasPrefix

`hasPrefix` reports whether s starts with prefix.

```text
{{ hasPrefix .spec.image "docker.io/" }}  → true
```

### hasSuffix

`hasSuffix` reports whether s ends with suffix.

```text
{{ hasSuffix .spec.host ".local" }}  → true
```

### join

`join` joins the elements of a list with sep between them.

```text
{{ join (podNames .pods) ", " }}  → "web-0, web-1, web-2"
```

### repeat

`repeat` returns s repeated count times.

```text
{{ repeat "-" 3 }}  → "---"
```

### replace

`replace` returns s with every occurrence of old replaced by new.

```text
{{ replace "payments-api" "-" "_" }}  → "payments_api"
```

### split

`split` splits a string by a separator into a slice. Combine with `index` to extract a specific element.

```text
{{ index (split "latest,stable,v2" ",") 0 }}  → "latest"
```

### toLower

`toLower` returns s in lower case.

```text
{{ toLower "Payments-API" }}  → "payments-api"
```

### toUpper

`toUpper` returns s in upper case.

```text
{{ toUpper .spec.env }}  → "PROD"
```

### trim

`trim` returns s without the leading and trailing characters in cutset.

```text
{{ trim .spec.path "/" }}  → "api"
```

### trimPrefix

`trimPrefix` returns s without prefix, if it starts with it.

```text
{{ trimPrefix .spec.image "docker.io/" }}
```

### trimSpace

`trimSpace` returns s without leading and trailing white space.

```text
{{ trimSpace .spec.name }}
```

### trimSuffix

`trimSuffix` returns s without suffix, if it ends with it.

```text
{{ trimSuffix .spec.host ".local" }}
```
