# Domain

Take domain names and URLs apart: the host of a URL, and a domain without its subdomain.

## Reference

| Function | Description |
|----------|-------------|
| [`domainBare`](#domainbare) | Is like `domainHost`, and also strips a leading `www.` subdomain. |
| [`domainHost`](#domainhost) | Strips surrounding whitespace, `https://` or `http://` prefix, and trailing `/`. |

## Functions

### domainBare

`domainBare` is like `domainHost`, and also strips a leading `www.` subdomain. Returns `""` for nil or non-string input. Idempotent.

```text
{{ domainBare "https://www.acme.com/" }}  → "acme.com"
```

### domainHost

`domainHost` strips surrounding whitespace, `https://` or `http://` prefix, and trailing `/`. Returns the bare hostname. Returns `""` for nil or non-string input. Idempotent — calling it on an already-clean hostname is a no-op.

```text
{{ domainHost "https://acme.example.com/" }}  → "acme.example.com"
```
