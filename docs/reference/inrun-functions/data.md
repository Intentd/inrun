# Data

Encode, hash and shape values: base64, a short SHA-256 for content-addressed names, slugs and truncation that keep a value valid as a Kubernetes name, and lookup tables written inline in an expression.

## Reference

| Function | Description |
|----------|-------------|
| [`fromBase64`](#frombase64) | Decodes a base64 string, such as a Secret's data value. |
| [`lookup`](#lookup) | Returns the value paired with a key in a flat list of alternating key/value pairs — the first match wins. |
| [`repoSlug`](#reposlug) | Extracts a Kubernetes-safe name from a repository reference — the last path segment (repo name), with a trailing `.git` stripped, then slugified. |
| [`sha256sum`](#sha256sum) | Returns the first 8 hex characters of the SHA-256 hash of s: a short, stable fingerprint for content-addressed names and change detection. |
| [`slugify`](#slugify) | Converts a string to a Kubernetes-safe name segment: lowercase, non-alphanumeric characters replaced with dashes, consecutive dashes collapsed, leading and trailing dashes trimmed. |
| [`toBase64`](#tobase64) | Base64-encodes s, as a Secret's data values are stored. |
| [`truncate`](#truncate) | Cuts s to at most maxLen characters with no suffix, so the result stays a valid Kubernetes name or label value. |

## Functions

### fromBase64

`fromBase64` decodes a base64 string, such as a Secret's data value. An error when s is not base64.

```text
{{ fromBase64 .secret.data.password }}  → "s3cr3t"
```

### lookup

`lookup` returns the value paired with a key in a flat list of alternating key/value pairs — the first match wins. Returns `""` if the key matches none, same as a missing field elsewhere. A trailing unpaired argument is ignored rather than erroring.

```text
{{ lookup "orders" "payments" "team-payments" "orders" "team-orders" }}  → "team-orders"
```

### repoSlug

`repoSlug` extracts a Kubernetes-safe name from a repository reference — the last path segment (repo name), with a trailing `.git` stripped, then slugified. Works with git URLs, org/repo shorthand, or a bare repo name — the platform team's repository enum values don't need to be pre-cleaned.

```text
{{ repoSlug "myorg/payments-api" }}                     → "payments-api"
{{ repoSlug "git@github.com:myorg/payments-api.git" }}  → "payments-api"
```

### sha256sum

`sha256sum` returns the first 8 hex characters of the SHA-256 hash of s: a short, stable fingerprint for content-addressed names and change detection.

```text
{{ sha256sum "log.level=debug" }}  → "728d4917"
```

### slugify

`slugify` converts a string to a Kubernetes-safe name segment: lowercase, non-alphanumeric characters replaced with dashes, consecutive dashes collapsed, leading and trailing dashes trimmed.

```text
{{ slugify "My Team / Platform" }}  → "my-team-platform"
```

### toBase64

`toBase64` base64-encodes s, as a Secret's data values are stored.

```text
{{ toBase64 "s3cr3t" }}  → "czNjcjN0"
```

### truncate

`truncate` cuts s to at most maxLen characters with no suffix, so the result stays a valid Kubernetes name or label value. For text people read, use ellipsis.

```text
{{ truncate "payments-api-production" 12 }}  → "payments-api"
```
