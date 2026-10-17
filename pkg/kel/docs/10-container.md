# 10 — Container Notes

Container notes extract information from Kubernetes Deployment or Pod objects — specifically from the `spec.template.spec.containers` array. They are designed to read data from child resources available under `.children.*`.

## What they navigate

A Kubernetes Deployment object under `.children.deployment` has this shape:

```
.children.deployment
  └── spec
        └── template
              └── spec
                    └── containers
                          └── [0]
                                ├── image
                                ├── env: [{name: "...", value: "..."}]
                                └── ports: [{containerPort: 8080}]
```

Container notes navigate this path safely — they return empty/false values rather than panicking when any segment is absent.

## Reference

### `containerImage`

Return the container image at the given index.

Keywords: container, image, deployment, pod, version, runtime

```yaml
# value: "{{ containerImage .children.deployment 0 }}"
# → "nginx:1.25"  (image of the first container)
```

Useful in status fields to surface what image is actually running in the child Deployment:

```yaml
status:
  fields:
    - path: runningImage
      value: "{{ containerImage .children.deployment 0 }}"
```

---

### `containerEnv`

Return the value of a named environment variable from the container at the given index.

Keywords: container, environment, env, variable, deployment, pod, config

```yaml
# value: "{{ containerEnv .children.deployment 0 \"APP_ENV\" }}"
# → "production"
```

Returns `""` when the variable is absent, the container index is out of range, or the object is missing.

Useful for comparing what environment a child container is configured with:

```yaml
status:
  fields:
    - path: deployedEnvironment
      value: "{{ containerEnv .children.deployment 0 \"APP_ENV\" }}"
```

---

### `containerPort`

Return `true` when the container at the given index exposes the specified port number.

Keywords: container, port, expose, deployment, pod, boolean, network

```yaml
# value: "{{ containerPort .children.deployment 0 8080 }}"
# → true   (if container[0].ports contains containerPort: 8080)
# → false  (otherwise)
```

Useful for verifying that a child Deployment is exposing the expected port before creating a Service to front it:

```yaml
status:
  fields:
    - path: port8080Exposed
      value: "{{ containerPort .children.deployment 0 8080 }}"
```

---

## Index convention

All container notes take a zero-based `index` parameter. For single-container Deployments (the most common case), always use `0`.

For multi-container pods, use the index that matches the container declaration order in the spec.

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `containerImage` | `(obj any, index int)` | `string` |
| `containerEnv` | `(obj any, index int, key string)` | `string` |
| `containerPort` | `(obj any, index int, port int)` | `bool` |

---

**↑ Back to** [Note Library README](README.md)
