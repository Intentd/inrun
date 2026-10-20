# ClusterRoleBinding

This declares one cluster-scoped ClusterRoleBinding to be managed by Inrun.

ClusterRoleBindings are cluster-scoped — no namespace field. Ownership is tracked via the inrun.dev/owner label.

Example:

```yaml
onCreate:
  clusterRoleBindings:
    - name: "{{ .metadata.name }}-crb"
      roleRef:
        name: "{{ .metadata.name }}-cluster-role"
        kind: ClusterRole
      subjects:
        - kind: ServiceAccount
          name: "{{ .metadata.name }}-sa"
          namespace: "{{ .metadata.namespace }}"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — ClusterRoleBinding name. Default when omitted: "{{ .metadata.name }}-crb"

---

### `labels`

Type: map

Labels — applied to ClusterRoleBinding metadata. Values support template expressions.

---

### `roleRef`

Type: object

RoleRef — the ClusterRole this binding grants. Required. kind defaults to "ClusterRole" when omitted (the only valid target for a ClusterRoleBinding).

```yaml
roleRef:
  name: "{{ .metadata.name }}-cluster-role"
```

---

### `subjects`

Type: list

Subjects — the users, groups, or ServiceAccounts this binding grants the role to. Required: at least one subject.

```yaml
subjects:
  - kind: ServiceAccount
    name: "{{ .metadata.name }}-sa"
    namespace: "{{ .metadata.namespace }}"
```

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile. Equivalent to declaring the same entry under both onCreate and onReconcile. When false (default), only runs on onCreate (idempotent create).

---

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `forEach`

Type: object

ForEach declares dynamic expansion over a list field. When set, one source declaration becomes N declarations — one per list element. .item and .\<as> are available in template expressions within this declaration.

```yaml
forEach:
  field: spec.regions
  as: region
```

---

### `forceConflict`

Type: boolean

ForceConflict, when true, sets Force: true when applying this resource, taking ownership of conflicting fields instead of returning a conflict error. Overrides the CRD-level ForceConflict setting.

---

## Quick reference

| YAML key | Type |
|---|---|
| `name` | string |
| `labels` | map |
| `roleRef` | object |
| `subjects` | list |
| `when` | list |
| `reconcile` | boolean |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
