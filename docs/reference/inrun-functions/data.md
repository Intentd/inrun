# Data Notes

Data encoding, hashing, and name-safety utilities. Used for Secret data encoding, content-addressed naming, JSON embedding, and generating Kubernetes-safe resource name segments from arbitrary string inputs.

## Reference

| Note | Description |
|------|-------------|
| `toBase64` | Base64-encode a string using standard encoding. |
| `fromBase64` | Decode a base64 string. |
| `toJSON` | Marshal any value to a JSON string. |
| `sha256sum` | Return the first 8 hex characters of the SHA256 hash of a string. |
| `truncateName` | Hard-truncate a string to at most `maxLen` characters with no suffix. |
| `slugify` | Convert a string to a Kubernetes-safe name segment: lowercase, non-alphanumeric characters replaced with dashes, consecutive dashes collapsed, leading and trailing dashes trimmed. |
| `repoSlug` | Extract a Kubernetes-safe name from a repository reference — the last path segment (repo name), with a trailing `. |
| `lookup` | Return the value paired with a key in a flat list of alternating key/value pairs — the first match wins. |

## Examples

```yaml
# toBase64
onCreate:
  secrets:
    - name: "{{ .metadata.name }}-config"
      data:
        endpoint: "{{ toBase64 .spec.endpoint }}"
        token: "{{ toBase64 .spec.token }}"

# fromBase64
status:
  fields:
    - path: endpoint
      value: "{{ fromBase64 (index .children.secret.data \"endpoint\") }}"

# toJSON
onCreate:
  configMaps:
    - name: "{{ .metadata.name }}-config"
      data:
        settings: "{{ toJSON .spec.settings }}"
        labels: "{{ toJSON .metadata.labels }}"

# sha256sum
# Content-addressed ConfigMap name — changes when content changes
onCreate:
  configMaps:
    - name: "config-{{ sha256sum .spec.config }}"
      data:
        content: "{{ .spec.config }}"

# Change-detection annotation
metadata:
  annotations:
    myorg.io/config-hash: "{{ sha256sum .spec.config }}"

# truncateName
# Keep the name under 50 chars before appending a fixed suffix
name: "{{ truncateName .spec.projectName 50 }}-deployment"

# Combine with slugify for safe, length-bounded names
name: "{{ truncateName (slugify .spec.teamName) 40 }}-operator"

# slugify
# Derive a safe resource name from a human-readable field
name: "{{ slugify .spec.teamName }}-operator"
# "My Team / Platform" → "my-team-platform-operator"

# Use with truncateName for long values
name: "{{ truncateName (slugify .spec.projectName) 52 }}-svc"

# repoSlug
# serve.name derives the CR's metadata.name from the repo the developer picked —
# no name field for them (or CI) to supply at all.
serve:
  enabled: true
  name: '{{ repoSlug .spec.repository }}'

# "myorg/payments-api"                    → "payments-api"
# "git@github.com:myorg/payments-api.git" → "payments-api"
# "https://github.com/myorg/payments-api" → "payments-api"

# lookup
# Derive team from the repository the developer picked — platform team
# curates both the repository enum and the mapping, developer picks neither.
notes:
  functions:
    - name: teamName
      expression: |
        {{ lookup .spec.repository
             "myorg/payments-api"  "team-payments"
             "myorg/orders-api"    "team-orders"
             "myorg/inventory-api" "team-inventory" }}
```
