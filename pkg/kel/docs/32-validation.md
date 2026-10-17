# 32 — Validation Notes

General-purpose input-format checks. An Serve form field is free text until something checks it — these are the checks a catalog author reaches for most often: is this actually an email, a git repository, a URL, a container image reference, well-formed JSON, a valid port number. Catching a malformed value at admission time, with a message built from the field's `label:`, beats the developer discovering it three steps downstream when ArgoCD or cert-manager fails to reconcile.

---

## Reference

### `isValidEmail`

Report whether a string is a single, bare email address — `user@example.com`, not a display-name form like `User <user@example.com>`.

Keywords: email, validate, valid, boolean, format, contact

```yaml
validation:
  rules:
    - field: "{{ isValidEmail .spec.ownerEmail }}"
      equals: "true"
      message: "Owner Email must be a valid email address"
      action: deny
```

```text
{{ isValidEmail "team@myorg.io"        }}  → true
{{ isValidEmail "Team <team@myorg.io>" }}  → false
{{ isValidEmail "not-an-email"         }}  → false
```

---

### `isValidGitRepository`

Report whether a string looks like a git repository URL — `https://`, `git://`, `ssh://`, or the SCP-like `git@host:org/repo.git` shorthand. This is a shape check, not a reachability check — it doesn't dial the host.

Keywords: git, repository, url, validate, valid, boolean, format, repo

```yaml
validation:
  rules:
    - field: "{{ isValidGitRepository .spec.repoURL }}"
      equals: "true"
      message: "Repository URL must be a valid git repository"
      action: deny
```

```text
{{ isValidGitRepository "https://github.com/myorg/payments" }}  → true
{{ isValidGitRepository "git@github.com:myorg/payments.git" }}  → true
{{ isValidGitRepository "not a repo"                          }}  → false
```

---

### `isValidURL`

Report whether a string is an absolute `http` or `https` URL with a non-empty host.

Keywords: url, http, https, validate, valid, boolean, format, webhook

```yaml
validation:
  rules:
    - field: "{{ isValidURL .spec.webhookUrl }}"
      equals: "true"
      message: "Webhook URL must be a valid http(s) URL"
      action: deny
```

```text
{{ isValidURL "https://example.com/webhook" }}  → true
{{ isValidURL "ftp://example.com"            }}  → false
{{ isValidURL "not a url"                     }}  → false
```

---

### `isValidImageRef`

Report whether a string looks like a valid container image reference: optional registry (with optional port), lowercase repo path, optional `:tag` or `@digest`. This is a pragmatic check, not a full implementation of the OCI distribution spec — enough to catch the common mistakes: a stray space, an uppercase letter, a missing tag separator.

Keywords: image, container, docker, registry, validate, valid, boolean, format, tag

```yaml
validation:
  rules:
    - field: "{{ isValidImageRef .spec.image }}"
      equals: "true"
      message: "Container Image must be a valid image reference"
      action: deny
```

```text
{{ isValidImageRef "myorg/app:v1.2.3"                       }}  → true
{{ isValidImageRef "registry.myorg.io:5000/team/app:latest" }}  → true
{{ isValidImageRef "MyOrg/App"                               }}  → false
```

---

### `isValidJSON`

Report whether a string is syntactically valid JSON. Useful for form fields that collect a raw JSON blob — a label selector, a structured config value — with no schema to validate them otherwise.

Keywords: json, validate, valid, boolean, format, syntax

```yaml
validation:
  rules:
    - field: "{{ isValidJSON .spec.serviceSelector }}"
      equals: "true"
      message: "Service Selector must be valid JSON"
      action: deny
```

```text
{{ isValidJSON "{\"app\": \"payments-api\"}" }}  → true
{{ isValidJSON "{not json"                    }}  → false
```

---

### `isValidPort`

Report whether a string parses as an integer in the valid TCP/UDP port range, 1–65535.

Keywords: port, network, validate, valid, boolean, format, number

```yaml
validation:
  rules:
    - field: "{{ isValidPort .spec.port }}"
      equals: "true"
      message: "Port must be between 1 and 65535"
      action: deny
```

```text
{{ isValidPort "8080"  }}  → true
{{ isValidPort "0"     }}  → false
{{ isValidPort "70000" }}  → false
```
