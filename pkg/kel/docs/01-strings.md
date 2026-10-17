# 01 — String Notes

String notes manipulate text values. They cover the cases that come up constantly when building resource names, labels, and field values from CR data.

## Reference

### `toLower`

Convert a string to all lowercase.

Keywords: string, case, lowercase, transform, text

```yaml
# value: "{{ toLower .spec.environment }}"
# production → production
# PRODUCTION → production
```

### `toUpper`

Convert a string to all uppercase.

Keywords: string, case, uppercase, transform, text

```yaml
# value: "{{ toUpper .spec.tier }}"
# PREMIUM → PREMIUM
# premium → PREMIUM
```

---

### `trimSpace`

Remove leading and trailing whitespace. Useful when spec fields may carry accidental spaces.

Keywords: string, trim, whitespace, clean, strip

```yaml
# value: "{{ trimSpace .spec.domain }}"
# "  example.com  " → "example.com"
```

---

### `trim`

Remove all leading and trailing occurrences of a cutset string.

Keywords: string, trim, cutset, strip, clean

```yaml
# value: "{{ trim .spec.tag \"/\" }}"
# "/images/nginx/" → "images/nginx"
```

---

### `trimPrefix`

Remove a specific prefix if present.

Keywords: string, trim, prefix, strip, remove

```yaml
# value: "{{ trimPrefix .metadata.name \"app-\" }}"
# "app-frontend" → "frontend"
```

### `trimSuffix`

Remove a specific suffix if present.

Keywords: string, trim, suffix, strip, remove
```yaml
# value: "{{ trimSuffix .spec.image \":latest\" }}"
# "nginx:latest" → "nginx"
```

---

### `hasPrefix`
Return `true` if the string starts with the given substring. Useful in `when:` conditions via a template expression.

Keywords: string, prefix, check, boolean, contains

```yaml
# value: '{{ hasPrefix .spec.image "gcr.io/" }}'
# "gcr.io/myproject/app:v1" → true
```

### `hasSuffix`

Return `true` if the string ends with the given substring.

Keywords: string, suffix, check, boolean, contains

```yaml
# value: '{{ hasSuffix .spec.image "v1" }}'
# "gcr.io/myproject/app:v1" → true
```

---

### `contains`

Return `true` if the string contains the substring.

Keywords: string, contains, substring, check, boolean, match

```yaml
# value: "{{ contains .spec.environment \"prod\" }}"
# "production" → true
# "staging"    → false
```

---

### `replace`

Replace all occurrences of `old` with `new` in `s`. This is `strings.ReplaceAll`.

Keywords: string, replace, substitute, transform, rewrite

```yaml
# value: "{{ replace .spec.domain \".\" \"-\" }}"
# "my.app.example.com" → "my-app-example-com"
```

Useful for turning domain names into DNS-valid Kubernetes resource names.

---

### `split`

Split a string by a separator into a slice. Combine with `index` to extract a specific element.

Keywords: string, split, separator, slice, list, parse

```yaml
# Extract the first tag from a comma-separated list
# value: "{{ index (split .spec.tags \",\") 0 }}"
# spec.tags: "latest,stable,v2" → "latest"
```

Returns an empty slice for empty input — no panic.

---

### `join`

Join a slice of strings into a single string with a separator.

Keywords: string, join, list, separator, concat, combine

```yaml
# value: "{{ join .spec.hosts \", \" }}"
# ["api.example.com", "admin.example.com"] → "api.example.com, admin.example.com"
```

---

### `repeat`

Repeat a string n times.

Keywords: string, repeat, duplicate, multiply

```yaml
# value: "{{ repeat \"ha\" 3 }}"
# → "hahaha"
```

---

### `camelToKebab`

Convert CamelCase or PascalCase to kebab-case. Useful when deriving Kubernetes resource names from Go type names.

Keywords: string, case, camel, kebab, transform, naming, kubernetes

```yaml
# value: "{{ camelToKebab .spec.controllerName }}"
# "WebsiteOperator" → "website-operator"
# "myAppName"       → "my-app-name"
```

---

### `concat`

Join any number of strings together with no separator. Useful for building domain names, resource name prefixes, or any value assembled from multiple parts.

Keywords: string, concat, join, combine, build, prefix, suffix, append

```yaml
# value: "{{ concat \"*.\" .spec.domain }}"
# spec.domain: "api.example.com" → "*.api.example.com"

# value: "{{ concat .metadata.name \"-\" .spec.tier }}"
# name: "webapp", tier: "prod" → "webapp-prod"
```

---

### `truncate`

Truncate a string to at most `n` characters. Appends `...` when truncated. Kubernetes labels have a 63-character limit — use this to stay within it.

Keywords: string, truncate, limit, length, kubernetes, label

```yaml
# value: "{{ truncate .metadata.name 63 }}"
# Names longer than 63 chars are clipped and get "..." appended.
# Names at or under 63 chars are returned unchanged.
```

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `toLower` | `(s string)` | `string` |
| `toUpper` | `(s string)` | `string` |
| `trimSpace` | `(s string)` | `string` |
| `trim` | `(s, cutset string)` | `string` |
| `trimPrefix` | `(s, prefix string)` | `string` |
| `trimSuffix` | `(s, suffix string)` | `string` |
| `hasPrefix` | `(s, prefix string)` | `bool` |
| `hasSuffix` | `(s, suffix string)` | `bool` |
| `contains` | `(s, substr string)` | `bool` |
| `replace` | `(s, old, new string)` | `string` |
| `split` | `(s, sep string)` | `[]string` |
| `join` | `(elems []string, sep string)` | `string` |
| `repeat` | `(s string, n int)` | `string` |
| `camelToKebab` | `(s string)` | `string` |
| `truncate` | `(s string, n int)` | `string` |
| `concat` | `(parts ...string)` | `string` |

---

**Next →** [02 — Math Notes](02-math.md)
