# Random

Generate cryptographically secure random values: hex, base64 and alphanumeric strings, and UUIDs. They return a different value on every call, so evaluate them once, when a secret is first created, and keep the result; evaluated on every reconcile, a password changes every time.

## Reference

| Function | Description |
|----------|-------------|
| [`randomAlphanumeric`](#randomalphanumeric) | Generates a cryptographically random alphanumeric string of exactly `n` characters. |
| [`randomBase64`](#randombase64) | Generates `n` random bytes and returns them as a URL-safe base64 string. |
| [`randomHex`](#randomhex) | Generates `n` random bytes and returns them as a hex-encoded string. |
| [`uuidv4`](#uuidv4) | Generates a random UUID v4 string in standard `8-4-4-4-12` hex format. |

## Functions

### randomAlphanumeric

`randomAlphanumeric` generates a cryptographically random alphanumeric string of exactly `n` characters. Characters are drawn from `[a-zA-Z0-9]`.

```text
{{ randomAlphanumeric 12 }}  → "k7Xm3pQs9vR2"
```

### randomBase64

`randomBase64` generates `n` random bytes and returns them as a URL-safe base64 string. The output length is approximately `ceil(n * 4/3)` characters.

```text
{{ randomBase64 16 }}  → "47H1Wtj_DBL6gy0hBE-49w=="
```

### randomHex

`randomHex` generates `n` random bytes and returns them as a hex-encoded string. The output is `2n` characters long.

```text
{{ randomHex 8 }}  → "a3f8b2c1d4e5f6a7"
```

### uuidv4

`uuidv4` generates a random UUID v4 string in standard `8-4-4-4-12` hex format. Same entropy as `randomHex 16`, formatted as a UUID for systems that expect that shape.

```text
{{ uuidv4 }}  → "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```
