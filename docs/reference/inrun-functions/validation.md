# Validation Notes

General-purpose input-format checks. An Serve form field is free text until something checks it — these are the checks a catalog author reaches for most often: is this actually an email, a git repository, a URL, a container image reference, well-formed JSON, a valid port number. Catching a malformed value at admission time, with a message built from the field's `label:`, beats the developer discovering it three steps downstream when ArgoCD or cert-manager fails to reconcile.

---

## Reference

| Note | Description |
|------|-------------|
| `isValidEmail` | Report whether a string is a single, bare email address — `user@example. |
| `isValidGitRepository` | Report whether a string looks like a git repository URL — `https://`, `git://`, `ssh://`, or the SCP-like `git@host:org/repo. |
| `isValidURL` | Report whether a string is an absolute `http` or `https` URL with a non-empty host. |
| `isValidImageRef` | Report whether a string looks like a valid container image reference: optional registry (with optional port), lowercase repo path, optional `:tag` or `@digest`. |
| `isValidJSON` | Report whether a string is syntactically valid JSON. |
| `isValidPort` | Report whether a string parses as an integer in the valid TCP/UDP port range, 1–65535. |

## Examples

```yaml
# isValidEmail
validation:
  rules:
    - field: "{{ isValidEmail .spec.ownerEmail }}"
      equals: "true"
      message: "Owner Email must be a valid email address"
      action: deny
{{ isValidEmail "team@myorg.io"        }}  → true
{{ isValidEmail "Team <team@myorg.io>" }}  → false
{{ isValidEmail "not-an-email"         }}  → false

# isValidGitRepository
validation:
  rules:
    - field: "{{ isValidGitRepository .spec.repoURL }}"
      equals: "true"
      message: "Repository URL must be a valid git repository"
      action: deny
{{ isValidGitRepository "https://github.com/myorg/payments" }}  → true
{{ isValidGitRepository "git@github.com:myorg/payments.git" }}  → true
{{ isValidGitRepository "not a repo"                          }}  → false

# isValidURL
validation:
  rules:
    - field: "{{ isValidURL .spec.webhookUrl }}"
      equals: "true"
      message: "Webhook URL must be a valid http(s) URL"
      action: deny
{{ isValidURL "https://example.com/webhook" }}  → true
{{ isValidURL "ftp://example.com"            }}  → false
{{ isValidURL "not a url"                     }}  → false

# isValidImageRef
validation:
  rules:
    - field: "{{ isValidImageRef .spec.image }}"
      equals: "true"
      message: "Container Image must be a valid image reference"
      action: deny
{{ isValidImageRef "myorg/app:v1.2.3"                       }}  → true
{{ isValidImageRef "registry.myorg.io:5000/team/app:latest" }}  → true
{{ isValidImageRef "MyOrg/App"                               }}  → false

# isValidJSON
validation:
  rules:
    - field: "{{ isValidJSON .spec.serviceSelector }}"
      equals: "true"
      message: "Service Selector must be valid JSON"
      action: deny
{{ isValidJSON "{\"app\": \"payments-api\"}" }}  → true
{{ isValidJSON "{not json"                    }}  → false

# isValidPort
validation:
  rules:
    - field: "{{ isValidPort .spec.port }}"
      equals: "true"
      message: "Port must be between 1 and 65535"
      action: deny
{{ isValidPort "8080"  }}  → true
{{ isValidPort "0"     }}  → false
{{ isValidPort "70000" }}  → false
```
