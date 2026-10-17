# Field Notes

Field notes are direct accessors for the common Kubernetes metadata fields that appear on every resource. They eliminate the need to navigate `metadata.*` paths manually and provide a consistent way to identify resources by name, namespace, UID, or version.

---

## Reference

| Note | Description |
|------|-------------|
| `resourceName` | Return `metadata. |
| `resourceNamespace` | Return `metadata. |
| `resourceUID` | Return `metadata. |
| `resourceVersion` | Return `metadata. |
| `creationTimestamp` | Return `metadata. |

## Examples

```yaml
# resourceName
# value: "{{ resourceName .children.deployment }}"  → "my-app"

# Include child name in a status field:
- path: deploymentName
  value: "{{ resourceName .children.deployment }}"

# resourceNamespace
# value: "{{ resourceNamespace .children.deployment }}"  → "production"

# Gate on namespace in status:
- path: deployedNamespace
  value: "{{ resourceNamespace .children.deployment }}"

# resourceUID
# value: "{{ resourceUID .children.deployment }}"
# → "4b3f8d21-8e3a-4f8c-b9d2-1a2b3c4d5e6f"

# resourceVersion
# value: "{{ resourceVersion .children.deployment }}"  → "14872"

# creationTimestamp
# value: "{{ creationTimestamp .children.deployment }}"  → "2024-01-15T10:30:00Z"

# Combine with time notes to compute age:
- path: deploymentAge
  value: "{{ timeSince (creationTimestamp .children.deployment) }}"
# Preferred — note syntax works everywhere
when:
  - field: "{{ resourceExists .children.deployment }}"
    equals: "true"
  - field: "{{ resourceNamespace .children.deployment }}"
    equals: "production"

status:
  fields:
    - path: childName
      value: "{{ resourceName .children.deployment }}"
    - path: childUID
      value: "{{ resourceUID .children.deployment }}"

# Avoid — raw path, panics if metadata absent
# field: children.deployment.metadata.name
```
