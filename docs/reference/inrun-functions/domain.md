# Domain Notes

Domain notes clean protocol-prefixed URL strings into bare hostnames. Use them in `normalize:` blocks where spec fields may arrive as `"https://acme.com/"`, `"http://acme.com"`, or already-clean `"acme.com"`. Both functions are pure, idempotent, and nil-safe.

## Reference

| Note | Description |
|------|-------------|
| `domainHost` | Strip surrounding whitespace, `https://` or `http://` prefix, and trailing `/`. |
| `domainBare` | Same as `domainHost`, and also strips a leading `www. |

## Examples

```yaml
# domainHost
normalize:
  spec:
    domain: '{{ domainHost .spec.domain }}'
# "https://acme.example.com/" → "acme.example.com"
# " http://acme.example.com " → "acme.example.com"
# "acme.example.com"          → "acme.example.com"
domain: '{{ trimSuffix (domainHost .spec.domain) ":8080" }}'
# "https://acme.example.com:8080/" → "acme.example.com"
{{ trimSuffix (trimPrefix (trimPrefix (trimSpace .spec.domain) "https://") "http://") "/" }}

# domainBare
normalize:
  spec:
    domain: '{{ domainBare .spec.domain }}'
# "https://www.acme.com/" → "acme.com"
# "https://acme.com/"     → "acme.com"
# "www.acme.com"          → "acme.com"
# "acme.com"              → "acme.com"
```
