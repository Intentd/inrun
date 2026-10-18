# Fields

Read the fields every Kubernetes object has: name, namespace, UID, resource version, creation timestamp, labels and annotations.

## Reference

| Function | Description |
|----------|-------------|
| [`creationTimestamp`](#creationtimestamp) | Returns `metadata.creationTimestamp` as an RFC3339 string. |
| [`resourceName`](#resourcename) | Returns `metadata.name`. |
| [`resourceNamespace`](#resourcenamespace) | Returns `metadata.namespace`. |
| [`resourceUID`](#resourceuid) | Returns `metadata.uid` — the unique identifier assigned by the API server. |
| [`resourceVersion`](#resourceversion) | Returns `metadata.resourceVersion` — the etcd revision string. |

## Functions

### creationTimestamp

`creationTimestamp` returns `metadata.creationTimestamp` as an RFC3339 string.

```text
{{ creationTimestamp .deployment }}  → "2026-01-15T10:30:00Z"
{{ timeSince (creationTimestamp .deployment) }}
```

### resourceName

`resourceName` returns `metadata.name`. Returns `""` when absent.

```text
{{ resourceName .deployment }}  → "web"
```

### resourceNamespace

`resourceNamespace` returns `metadata.namespace`. Returns `""` for cluster-scoped resources.

```text
{{ resourceNamespace .deployment }}  → "payments"
```

### resourceUID

`resourceUID` returns `metadata.uid` — the unique identifier assigned by the API server. Stable across renames; changes on delete-and-recreate.

```text
{{ resourceUID .deployment }}  → "4b3f8d21-8e3a-4f8c-b9d2-1a2b3c4d5e6f"
```

### resourceVersion

`resourceVersion` returns `metadata.resourceVersion` — the etcd revision string. Increments on every write. Useful for detecting whether an object has changed since last observation.

```text
{{ resourceVersion .deployment }}  → "14872"
```
