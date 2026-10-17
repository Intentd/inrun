# 06 — Random Notes

Random notes generate cryptographically secure random values. They are designed for one use case: generating secrets that should be created once and never changed.

## The critical rule: always use `once: true`

Random notes are **not idempotent**. Every call to `randomAlphanumeric`, `randomHex`, or `randomBase64` produces a different value. Without `once: true`, a new password is generated on every reconcile cycle — every 30 seconds — breaking every application that reads it.

```yaml
# WRONG — password changes every reconcile
secrets:
  - name: "{{ .metadata.name }}-creds"
    data:
      password: "{{ randomAlphanumeric 32 }}"

# CORRECT — password generated once, never changed
secrets:
  - name: "{{ .metadata.name }}-creds"
    once: true
    data:
      password: "{{ randomAlphanumeric 32 }}"
```

The `once: true` flag makes the reconciler skip Secret creation if the Secret already exists in Kubernetes. The random note is only evaluated when the Secret does not yet exist.

## Reference

### `uuidv4`

Generate a random UUID v4 string in standard `8-4-4-4-12` hex format. Same entropy as `randomHex 16`, formatted as a UUID for systems that expect that shape.

Keywords: random, uuid, secret, token, id, identifier, generate, crypto

```yaml
# value: "{{ uuidv4 }}"
# → "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```

**Use for:** Gateway API tokens, OAuth client IDs, correlation IDs, any consumer that expects UUID-shaped values.

---

### `randomAlphanumeric`

Generate a cryptographically random alphanumeric string of exactly `n` characters. Characters are drawn from `[a-zA-Z0-9]`.

Keywords: random, secret, password, alphanumeric, generate, crypto, credentials

```yaml
# value: "{{ randomAlphanumeric 32 }}"
# → "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5"
```

**Use for:** database passwords, admin credentials, application secrets that must be readable (no special characters).

`n` should be at least 16 for security. For passwords, 32 is a reasonable default.

---

### `randomHex`

Generate `n` random bytes and return them as a hex-encoded string. The output is `2n` characters long.

Keywords: random, secret, token, hex, generate, crypto, api-key, session

```yaml
# value: "{{ randomHex 16 }}"
# 16 bytes → 32 hex chars
# → "a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5"
```

**Use for:** API keys, session tokens, CSRF tokens, database encryption keys.

The hex encoding uses lowercase characters (`0-9a-f`).

---

### `randomBase64`

Generate `n` random bytes and return them as a URL-safe base64 string. The output length is approximately `ceil(n * 4/3)` characters.

Keywords: random, secret, jwt, base64, generate, crypto, signing, hmac, oauth

```yaml
# value: "{{ randomBase64 32 }}"
# 32 bytes → ~43 base64 chars
# → "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5zP..."
```

**Use for:** JWT secrets, HMAC signing keys, cookie secrets, OAuth client secrets.

Uses URL-safe encoding (`-` and `_` instead of `+` and `/`) — safe in HTTP headers and URL parameters without additional encoding.

---

## Rotation with `rotateAfter`

Random notes can be combined with `rotateAfter` to periodically regenerate credentials:

```yaml
secrets:
  - name: "{{ .metadata.name }}-api-key"
    once: true
    rotateAfter: "90d"
    data:
      apiKey: "{{ randomHex 32 }}"
```

When the Secret's `generated-at` annotation is older than `90d`, Inrun deletes it and re-creates it with a fresh value. The random note is evaluated again during re-creation.

---

## Source of randomness

All three notes use Go's `crypto/rand` package. This reads from the operating system's cryptographically secure random source (`/dev/urandom` on Linux). They will panic if the system random source is unavailable — this should never happen in a normally operating container.

---

## Quick reference

| Note | Signature | Output length | Use for |
|------|-----------|---------------|---------|
| `uuidv4` | `()` | 36 chars | UUID-shaped tokens, correlation IDs |
| `randomAlphanumeric` | `(n int)` | exactly `n` chars | Readable passwords |
| `randomHex` | `(n int)` | `2n` chars | API keys, tokens |
| `randomBase64` | `(n int)` | `~n*4/3` chars | JWT/HMAC secrets |

All four require `once: true` on the enclosing secret declaration.

---

**Next →** [07 — Collection Notes](07-collections.md)
