# Random Notes

Random notes generate cryptographically secure random values. They are designed for one use case: generating secrets that should be created once and never changed.

## Reference

| Note | Description |
|------|-------------|
| `uuidv4` | Generate a random UUID v4 string in standard `8-4-4-4-12` hex format. |
| `randomAlphanumeric` | Generate a cryptographically random alphanumeric string of exactly `n` characters. |
| `randomHex` | Generate `n` random bytes and return them as a hex-encoded string. |
| `randomBase64` | Generate `n` random bytes and return them as a URL-safe base64 string. |

## Examples

```yaml
# uuidv4
# value: "{{ uuidv4 }}"
# → "f47ac10b-58cc-4372-a567-0e02b2c3d479"

# randomAlphanumeric
# value: "{{ randomAlphanumeric 32 }}"
# → "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5"

# randomHex
# value: "{{ randomHex 16 }}"
# 16 bytes → 32 hex chars
# → "a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5"

# randomBase64
# value: "{{ randomBase64 32 }}"
# 32 bytes → ~43 base64 chars
# → "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5zP..."
secrets:
  - name: "{{ .metadata.name }}-api-key"
    once: true
    rotateAfter: "90d"
    data:
      apiKey: "{{ randomHex 32 }}"
```
