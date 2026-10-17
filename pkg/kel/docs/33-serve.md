# 33 — Serve Provenance Notes

Serve provenance notes read the three gateway annotations stamped on every CR the Gateway API produces. They let template authors write routing-aware status fields, conditional blocks, and response payloads without hard-coding annotation keys.

---

## Reference

### `getServeTarget`

Return the `inrun.dev/serve-target` annotation — the serve target name used when the CR was submitted. Returns `""` when the annotation is absent (CR was not submitted via the Gateway API).

Keywords: serve, gateway, target, annotation, provenance, routing, string

```yaml
# value: "{{ getServeTarget . }}"  → "smartapp"

# Expose the target in a status field:
- path: serveTarget
  value: "{{ getServeTarget . }}"
```

---

### `getServeAlias`

Return the `inrun.dev/serve-alias` annotation — the alias name used when the CR was submitted, if any. Returns `""` when the primary target was used directly (not an alias).

Keywords: serve, gateway, alias, annotation, provenance, routing, string

```yaml
# value: "{{ getServeAlias . }}"  → "public"   (empty when primary target used)

# Expose the alias alongside the target:
- path: serveAlias
  value: "{{ getServeAlias . }}"
```

---

### `getServeSource`

Return the `inrun.dev/serve-source` annotation — the delivery mechanism that submitted the CR. Returns `""` for direct Gateway API calls. Set by webhook integrations (known values: `github`, `gitlab`, `slack`, `pagerduty`, `generic`).

Keywords: serve, gateway, source, webhook, annotation, provenance, string

```yaml
# value: "{{ getServeSource . }}"  → "github"

# Gate a status message on source:
- path: triggeredBy
  value: "{{ getServeSource . | default \"manual\" }}"
```

---

### `hasServeTarget`

Report whether the `inrun.dev/serve-target` annotation is present and non-empty. Returns `true` for any CR submitted via the Gateway API.

Keywords: serve, gateway, target, annotation, boolean, presence

```yaml
# value: "{{ hasServeTarget . }}"  → "true"

# Conditional block in a response payload:
when:
  - field: "{{ hasServeTarget . }}"
    equals: "true"
```

---

### `hasServeAlias`

Report whether the `inrun.dev/serve-alias` annotation is set — i.e., the CR was reached via a named alias rather than the primary target.

Keywords: serve, gateway, alias, annotation, boolean, routing

```yaml
# value: "{{ hasServeAlias . }}"  → "true"

# Conditional display in a status field:
- path: routedVia
  value: "{{ if hasServeAlias . }}alias:{{ getServeAlias . }}{{ else }}direct{{ end }}"
```

---

### `hasServeSource`

Report whether the `inrun.dev/serve-source` annotation is set — i.e., the CR arrived via a webhook source integration rather than a direct API call.

Keywords: serve, gateway, source, webhook, annotation, boolean

```yaml
# value: "{{ hasServeSource . }}"  → "true"

# Include source in a notification field:
- path: triggerSource
  value: "{{ if hasServeSource . }}{{ getServeSource . }}{{ else }}api{{ end }}"
```

---

### `isDirectApply`

Report whether the CR was NOT submitted via the Gateway API — i.e. it arrived via `kubectl`, CI direct apply, or any other non-gateway path. Returns `true` only when none of the three provenance annotations (`serve-target`, `serve-alias`, `serve-source`) are present. Use this to gate stricter validation rules, different mutation defaults, or reconciler branches that handle raw applies separately from gateway-classified intent.

Keywords: serve, gateway, direct, kubectl, annotation, boolean, provenance

```yaml
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

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `getServeTarget` | `(obj any)` | `string` |
| `getServeAlias` | `(obj any)` | `string` |
| `getServeSource` | `(obj any)` | `string` |
| `hasServeTarget` | `(obj any)` | `bool` |
| `hasServeAlias` | `(obj any)` | `bool` |
| `hasServeSource` | `(obj any)` | `bool` |
| `isDirectApply` | `(obj any)` | `bool` |

---

**← Back to** [32 — Validation Notes](32-validation.md)
