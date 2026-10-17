# Serve Provenance Notes

Serve provenance notes read the three gateway annotations stamped on every CR the Gateway API produces. They let template authors write routing-aware status fields, conditional blocks, and response payloads without hard-coding annotation keys.

---

## Reference

| Note | Description |
|------|-------------|
| `getServeTarget` | Return the `inrun. |
| `getServeAlias` | Return the `inrun. |
| `getServeSource` | Return the `inrun. |
| `hasServeTarget` | Report whether the `inrun. |
| `hasServeAlias` | Report whether the `inrun. |
| `hasServeSource` | Report whether the `inrun. |
| `isDirectApply` | Report whether the CR was NOT submitted via the Gateway API — i. |

## Examples

```yaml
# getServeTarget
# value: "{{ getServeTarget . }}"  → "smartapp"

# Expose the target in a status field:
- path: serveTarget
  value: "{{ getServeTarget . }}"

# getServeAlias
# value: "{{ getServeAlias . }}"  → "public"   (empty when primary target used)

# Expose the alias alongside the target:
- path: serveAlias
  value: "{{ getServeAlias . }}"

# getServeSource
# value: "{{ getServeSource . }}"  → "github"

# Gate a status message on source:
- path: triggeredBy
  value: "{{ getServeSource . | default \"manual\" }}"

# hasServeTarget
# value: "{{ hasServeTarget . }}"  → "true"

# Conditional block in a response payload:
when:
  - field: "{{ hasServeTarget . }}"
    equals: "true"

# hasServeAlias
# value: "{{ hasServeAlias . }}"  → "true"

# Conditional display in a status field:
- path: routedVia
  value: "{{ if hasServeAlias . }}alias:{{ getServeAlias . }}{{ else }}direct{{ end }}"

# hasServeSource
# value: "{{ hasServeSource . }}"  → "true"

# Include source in a notification field:
- path: triggerSource
  value: "{{ if hasServeSource . }}{{ getServeSource . }}{{ else }}api{{ end }}"

# isDirectApply
# value: "{{ isDirectApply . }}"  → "true" when no serve-target annotation is present

# Enforce stricter image policy for direct applies:
validation:
  rules:
    - field: spec.image
      prefix: "myorg/"
      message: "direct applies must use the internal registry"
      action: deny
      when:
        - field: "{{ isDirectApply . }}"
          equals: "true"
```
