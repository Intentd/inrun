# 15 — Field Notes

Field notes are direct accessors for the common Kubernetes metadata fields that appear on every resource. They eliminate the need to navigate `metadata.*` paths manually and provide a consistent way to identify resources by name, namespace, UID, or version.

---

## Reference

### `resourceName`

Return `metadata.name`. Returns `""` when absent.

Keywords: kubernetes, metadata, name, field, identity, resource

```yaml
# value: "{{ resourceName .children.deployment }}"  → "my-app"

# Include child name in a status field:
- path: deploymentName
  value: "{{ resourceName .children.deployment }}"
```

---

### `resourceNamespace`

Return `metadata.namespace`. Returns `""` for cluster-scoped resources.

Keywords: kubernetes, metadata, namespace, field, scope, resource

```yaml
# value: "{{ resourceNamespace .children.deployment }}"  → "production"

# Gate on namespace in status:
- path: deployedNamespace
  value: "{{ resourceNamespace .children.deployment }}"
```

---

### `resourceUID`

Return `metadata.uid` — the unique identifier assigned by the API server. Stable across renames; changes on delete-and-recreate.

Keywords: kubernetes, metadata, uid, identifier, unique, resource

```yaml
# value: "{{ resourceUID .children.deployment }}"
# → "4b3f8d21-8e3a-4f8c-b9d2-1a2b3c4d5e6f"
```

---

### `resourceVersion`

Return `metadata.resourceVersion` — the etcd revision string. Increments on every write. Useful for detecting whether an object has changed since last observation.

Keywords: kubernetes, metadata, version, revision, etcd, change

```yaml
# value: "{{ resourceVersion .children.deployment }}"  → "14872"
```

---

### `creationTimestamp`

Return `metadata.creationTimestamp` as an RFC3339 string.

Keywords: kubernetes, metadata, timestamp, created, time, age, rfc3339

```yaml
# value: "{{ creationTimestamp .children.deployment }}"  → "2024-01-15T10:30:00Z"

# Combine with time notes to compute age:
- path: deploymentAge
  value: "{{ timeSince (creationTimestamp .children.deployment) }}"
```

---

## Notes are the single navigation model

Field notes (and all other Kubernetes notes) work in both `value:` fields and `field:` conditions via template syntax. This is the idiomatic pattern — avoid raw dot-path navigation:

```yaml
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

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `resourceName` | `(obj any)` | `string` |
| `resourceNamespace` | `(obj any)` | `string` |
| `resourceUID` | `(obj any)` | `string` |
| `resourceVersion` | `(obj any)` | `string` |
| `creationTimestamp` | `(obj any)` | `string` |

---

**← Back to** [14 — Service Notes](14-service.md)
