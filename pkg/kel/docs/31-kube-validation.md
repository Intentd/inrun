# 31 — Kubernetes Validation Notes

Kubernetes-standard label and annotation format checks, exposed as notes. Kubernetes already enforces this format at the API server — these notes expose the same check so a `validation.rules` entry can gate on it directly, at admission time, with an Inrun-native message instead of the API server's raw rejection, and so `inrun simulate` (which doesn't run full structural schema validation) can catch it too.

---

## Reference

### `isValidLabelValue`

Report whether a string is a syntactically valid Kubernetes label value: max 63 characters, alphanumeric, may contain `-`, `_`, `.`, and must start and end with an alphanumeric character. An empty string is valid — Kubernetes treats an absent label value as permitted.

Keywords: kubernetes, label, value, validate, valid, boolean, format

```yaml
validation:
  rules:
    - field: "{{ isValidLabelValue (getLabel . \"team\") }}"
      equals: "true"
      message: "Team label must be a valid Kubernetes label value"
      action: deny
```

```text
{{ isValidLabelValue "team-payments" }}  → true
{{ isValidLabelValue ""              }}  → true
{{ isValidLabelValue "team payments" }}  → false
{{ isValidLabelValue "-leading-dash" }}  → false
```

---

### `isValidLabelKey`

Report whether a string is a syntactically valid Kubernetes label key: `[prefix/]name` — `name` is alphanumeric (max 63 chars, may contain `-`, `_`, `.`), and `prefix`, if present, must be a valid DNS subdomain (max 253 chars).

Keywords: kubernetes, label, key, validate, valid, boolean, format, prefix

```yaml
validation:
  rules:
    - field: "{{ isValidLabelKey .spec.customLabelKey }}"
      equals: "true"
      message: "spec.customLabelKey is not a valid Kubernetes label key"
      action: deny
```

```text
{{ isValidLabelKey "tier"                     }}  → true
{{ isValidLabelKey "platform.myorg.io/tier"   }}  → true
{{ isValidLabelKey "bad key"                  }}  → false
```

---

### `isValidAnnotationKey`

Report whether a string is a syntactically valid Kubernetes annotation key — the same `[prefix/]name` format as a label key. Annotation values have no Kubernetes format restriction of their own (that's what distinguishes them from labels), so there is no `isValidAnnotationValue` note.

Keywords: kubernetes, annotation, key, validate, valid, boolean, format, prefix

```yaml
validation:
  rules:
    - field: "{{ isValidAnnotationKey .spec.customAnnotationKey }}"
      equals: "true"
      message: "spec.customAnnotationKey is not a valid Kubernetes annotation key"
      action: deny
```

```text
{{ isValidAnnotationKey "platform.myorg.io/jira-ticket" }}  → true
{{ isValidAnnotationKey "bad key"                       }}  → false
```

---

### `isDNS1123Subdomain`

Report whether a string is a syntactically valid DNS-1123 subdomain: lowercase alphanumeric segments (may contain `-`) separated by `.`, max 253 characters. This is the format Kubernetes requires for object names in most resource types, and for the prefix portion of a label/annotation key.

Keywords: kubernetes, dns, subdomain, name, validate, valid, boolean, format

```yaml
validation:
  rules:
    - field: "{{ isDNS1123Subdomain .spec.hostname }}"
      equals: "true"
      message: "spec.hostname must be a valid DNS subdomain"
      action: deny
```

```text
{{ isDNS1123Subdomain "my-app.example.com" }}  → true
{{ isDNS1123Subdomain "My_App"             }}  → false
{{ isDNS1123Subdomain ""                   }}  → false
```
