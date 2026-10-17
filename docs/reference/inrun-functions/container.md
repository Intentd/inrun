# Container Notes

Container notes extract information from Kubernetes Deployment or Pod objects — specifically from the `spec.template.spec.containers` array. They are designed to read data from child resources available under `.children.*`.

## Reference

| Note | Description |
|------|-------------|
| `containerImage` | Return the container image at the given index. |
| `containerEnv` | Return the value of a named environment variable from the container at the given index. |
| `containerPort` | Return `true` when the container at the given index exposes the specified port number. |

## Examples

```yaml
# containerImage
# value: "{{ containerImage .children.deployment 0 }}"
# → "nginx:1.25"  (image of the first container)
status:
  fields:
    - path: runningImage
      value: "{{ containerImage .children.deployment 0 }}"

# containerEnv
# value: "{{ containerEnv .children.deployment 0 \"APP_ENV\" }}"
# → "production"
status:
  fields:
    - path: deployedEnvironment
      value: "{{ containerEnv .children.deployment 0 \"APP_ENV\" }}"

# containerPort
# value: "{{ containerPort .children.deployment 0 8080 }}"
# → true   (if container[0].ports contains containerPort: 8080)
# → false  (otherwise)
status:
  fields:
    - path: port8080Exposed
      value: "{{ containerPort .children.deployment 0 8080 }}"
```
