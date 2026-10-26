# Validation

Check that free text is the kind of value it claims to be: an email address, a URL, JSON, a port number. A form field is free text until something checks it; catching a malformed value early, with a clear message, beats finding out downstream.

Check references to artifacts the tools around Kubernetes consume: a git repository (Argo CD, Flux) and a container image reference (registries). Catching a malformed one early beats finding out when the tool fails to reconcile it.

## Reference

| Function | Description |
|----------|-------------|
| [`isValidEmail`](#isvalidemail) | Reports whether s is a single, bare email address — "user@example.com", not a display-name form like "User &lt;user@example.com&gt;". |
| [`isValidGitRepository`](#isvalidgitrepository) | Reports whether s looks like a git repository URL — https://, git://, ssh://, or the SCP-like "git@host:org/repo.git" shorthand. |
| [`isValidImageRef`](#isvalidimageref) | Reports whether s looks like a valid container image reference: optional registry, lowercase repo path, optional :tag or @digest. |
| [`isValidJSON`](#isvalidjson) | Reports whether s is syntactically valid JSON. |
| [`isValidPort`](#isvalidport) | Reports whether s parses as an integer in the valid TCP/UDP port range, 1–65535. |
| [`isValidURL`](#isvalidurl) | Reports whether s is an absolute http or https URL with a non-empty host. |

## Functions

### isValidEmail

`isValidEmail` reports whether s is a single, bare email address — "user@example.com", not a display-name form like "User &lt;user@example.com&gt;".

```text
{{ isValidEmail "team@myorg.io" }}  → true
{{ isValidEmail "not-an-email"  }}  → false
```

### isValidGitRepository

`isValidGitRepository` reports whether s looks like a git repository URL — https://, git://, ssh://, or the SCP-like "git@host:org/repo.git" shorthand. This is a shape check, not a reachability check — it doesn't dial the host.

```text
{{ isValidGitRepository "https://github.com/myorg/payments"    }}  → true
{{ isValidGitRepository "git@github.com:myorg/payments.git"    }}  → true
{{ isValidGitRepository "not a repo"                            }}  → false
```

### isValidImageRef

`isValidImageRef` reports whether s looks like a valid container image reference: optional registry, lowercase repo path, optional :tag or @digest.

```text
{{ isValidImageRef "myorg/app:v1.2.3"                    }}  → true
{{ isValidImageRef "registry.myorg.io:5000/team/app:latest" }}  → true
{{ isValidImageRef "MyOrg/App"                            }}  → false
```

### isValidJSON

`isValidJSON` reports whether s is syntactically valid JSON.

```text
{{ isValidJSON "{\"app\": \"payments-api\"}" }}  → true
{{ isValidJSON "{not json"                    }}  → false
```

### isValidPort

`isValidPort` reports whether s parses as an integer in the valid TCP/UDP port range, 1–65535.

```text
{{ isValidPort "8080"  }}  → true
{{ isValidPort "0"     }}  → false
{{ isValidPort "70000" }}  → false
```

### isValidURL

`isValidURL` reports whether s is an absolute http or https URL with a non-empty host.

```text
{{ isValidURL "https://example.com/webhook" }}  → true
{{ isValidURL "ftp://example.com"            }}  → false
{{ isValidURL "not a url"                     }}  → false
```
