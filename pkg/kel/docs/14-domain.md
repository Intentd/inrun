# 14 — Domain Notes

Domain notes clean protocol-prefixed URL strings into bare hostnames. Use them in `normalize:` blocks where spec fields may arrive as `"https://acme.com/"`, `"http://acme.com"`, or already-clean `"acme.com"`. Both functions are pure, idempotent, and nil-safe.

## Reference

### `domainHost`

Strip surrounding whitespace, `https://` or `http://` prefix, and trailing `/`. Returns the bare hostname. Returns `""` for nil or non-string input. Idempotent — calling it on an already-clean hostname is a no-op.

Keywords: domain, string, url, hostname, protocol, normalize, strip

```yaml
normalize:
  spec:
    domain: '{{ domainHost .spec.domain }}'
# "https://acme.example.com/" → "acme.example.com"
# " http://acme.example.com " → "acme.example.com"
# "acme.example.com"          → "acme.example.com"
```

To strip a port suffix, pipe through `trimSuffix`:

```yaml
domain: '{{ trimSuffix (domainHost .spec.domain) ":8080" }}'
# "https://acme.example.com:8080/" → "acme.example.com"
```

Equivalent long form:

```
{{ trimSuffix (trimPrefix (trimPrefix (trimSpace .spec.domain) "https://") "http://") "/" }}
```

---

### `domainBare`

Same as `domainHost`, and also strips a leading `www.` subdomain. Returns `""` for nil or non-string input. Idempotent.

Keywords: domain, string, www, subdomain, hostname, normalize, strip

```yaml
normalize:
  spec:
    domain: '{{ domainBare .spec.domain }}'
# "https://www.acme.com/" → "acme.com"
# "https://acme.com/"     → "acme.com"
# "www.acme.com"          → "acme.com"
# "acme.com"              → "acme.com"
```

---

## Quick reference

| Note | Accepts | Returns | Use in |
|------|---------|---------|--------|
| `domainHost` | `string` or nil | `string` | `normalize:`, `status:`, `when:` |
| `domainBare` | `string` or nil | `string` | `normalize:`, `status:`, `when:` |
