# Kubernetes validation

Check values against the rules the API server applies to names and metadata: label keys and values, annotation keys, and DNS-1123 subdomains, so a bad value is caught with a clear message before the API server rejects it.

## Reference

| Function | Description |
|----------|-------------|
| [`isDNS1123Subdomain`](#isdns1123subdomain) | Reports whether a string is a syntactically valid DNS-1123 subdomain: lowercase alphanumeric segments (may contain `-`) separated by `.`, max 253 characters. |
| [`isValidAnnotationKey`](#isvalidannotationkey) | Reports whether a string is a syntactically valid Kubernetes annotation key — the same `[prefix/]name` format as a label key. |
| [`isValidLabelKey`](#isvalidlabelkey) | Reports whether a string is a syntactically valid Kubernetes label key: `[prefix/]name` — `name` is alphanumeric (max 63 chars, may contain `-`, `_`, `.`), and `prefix`, if present, must be a valid DNS subdomain (max 253 chars). |
| [`isValidLabelValue`](#isvalidlabelvalue) | Reports whether a string is a syntactically valid Kubernetes label value: max 63 characters, alphanumeric, may contain `-`, `_`, `.`, and must start and end with an alphanumeric character. |

## Functions

### isDNS1123Subdomain

`isDNS1123Subdomain` reports whether a string is a syntactically valid DNS-1123 subdomain: lowercase alphanumeric segments (may contain `-`) separated by `.`, max 253 characters. This is the format Kubernetes requires for object names in most resource types, and for the prefix portion of a label/annotation key.

```text
{{ isDNS1123Subdomain "api.example.com" }}  → true
{{ isDNS1123Subdomain "API_Example" }}      → false
```

### isValidAnnotationKey

`isValidAnnotationKey` reports whether a string is a syntactically valid Kubernetes annotation key — the same `[prefix/]name` format as a label key. Annotation values have no Kubernetes format restriction of their own (that's what distinguishes them from labels), so there is no `isValidAnnotationValue` function.

```text
{{ isValidAnnotationKey "example.com/owner" }}  → true
```

### isValidLabelKey

`isValidLabelKey` reports whether a string is a syntactically valid Kubernetes label key: `[prefix/]name` — `name` is alphanumeric (max 63 chars, may contain `-`, `_`, `.`), and `prefix`, if present, must be a valid DNS subdomain (max 253 chars).

```text
{{ isValidLabelKey "app.kubernetes.io/name" }}  → true
```

### isValidLabelValue

`isValidLabelValue` reports whether a string is a syntactically valid Kubernetes label value: max 63 characters, alphanumeric, may contain `-`, `_`, `.`, and must start and end with an alphanumeric character. An empty string is valid — Kubernetes treats an absent label value as permitted.

```text
{{ isValidLabelValue "team-payments" }}  → true
{{ isValidLabelValue "team payments" }}  → false
```
