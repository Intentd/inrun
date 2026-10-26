# Serve

Read the provenance a CR carries when Inrun's gateway created it: the serve target and alias it came through, and whether it was applied directly instead.

## Reference

| Function | Description |
|----------|-------------|
| [`getServeAlias`](#getservealias) | Returns the `inrun.dev/serve-alias` annotation — the alias name used when the CR was submitted, if any. |
| [`getServeTarget`](#getservetarget) | Returns the `inrun.dev/serve-target` annotation — the serve target name used when the CR was submitted. |
| [`hasServeAlias`](#hasservealias) | Reports whether the `inrun.dev/serve-alias` annotation is set — i.e., the CR was reached via a named alias rather than the primary target. |
| [`hasServeTarget`](#hasservetarget) | Reports whether the `inrun.dev/serve-target` annotation is present and non-empty. |
| [`isDirectApply`](#isdirectapply) | Reports whether the CR was NOT submitted via the Gateway API — i.e. it arrived via `kubectl`, CI direct apply, or any other non-gateway path. |

## Functions

### getServeAlias

`getServeAlias` returns the `inrun.dev/serve-alias` annotation — the alias name used when the CR was submitted, if any. Returns `""` when the primary target was used directly (not an alias).

```text
# value: "{{ getServeAlias . }}"  → "public"   (empty when primary target used)

# Expose the alias alongside the target:
- path: serveAlias
  value: "{{ getServeAlias . }}"
```

### getServeTarget

`getServeTarget` returns the `inrun.dev/serve-target` annotation — the serve target name used when the CR was submitted. Returns `""` when the annotation is absent (CR was not submitted via the Gateway API).

```text
# value: "{{ getServeTarget . }}"  → "smartapp"

# Expose the target in a status field:
- path: serveTarget
  value: "{{ getServeTarget . }}"
```

### hasServeAlias

`hasServeAlias` reports whether the `inrun.dev/serve-alias` annotation is set — i.e., the CR was reached via a named alias rather than the primary target.

```text
# value: "{{ hasServeAlias . }}"  → "true"

# Conditional display in a status field:
- path: routedVia
  value: "{{ if hasServeAlias . }}alias:{{ getServeAlias . }}{{ else }}direct{{ end }}"
```

### hasServeTarget

`hasServeTarget` reports whether the `inrun.dev/serve-target` annotation is present and non-empty. Returns `true` for any CR submitted via the Gateway API.

```text
# value: "{{ hasServeTarget . }}"  → "true"

# Conditional block in a response payload:
when:
  - field: "{{ hasServeTarget . }}"
    equals: "true"
```

### isDirectApply

`isDirectApply` reports whether the CR was NOT submitted via the Gateway API — i.e. it arrived via `kubectl`, CI direct apply, or any other non-gateway path. Returns `true` only when none of the three provenance annotations (`serve-target`, `serve-alias`, `serve-source`) are present. Use this to gate stricter validation rules, different mutation defaults, or reconciler branches that handle raw applies separately from gateway-classified intent.

```text
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
