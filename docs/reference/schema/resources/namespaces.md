# Namespace

This declares one Namespace to be managed by Inrun.

Example:

```yaml
onCreate:
  namespaces:
    - name: "{{ .metadata.name }}-ns"
      labels:
        app: "{{ .metadata.name }}"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — Namespace name. Default when omitted: "{{ .metadata.name }}-ns"

---

### `finalizers`

Type: list

Finalizers — additional Kubernetes finalizers to add to the Namespace, beyond the standard "kubernetes" finalizer. The Namespace stays in Terminating state until every finalizer is removed by its owning controller, so only add one if something must run cleanup before this Namespace can actually be deleted.

```yaml
finalizers:
  - "{{ .metadata.name }}/cleanup"
```

---

### `labels`

Type: map

Labels — applied to Namespace metadata. Values support template expressions.

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile. Equivalent to declaring the same entry under both onCreate and onReconcile. When false (default), only runs on onCreate (idempotent create).

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

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `forceConflict`

Type: boolean

ForceConflict, when true, sets Force: true when applying this resource, taking ownership of conflicting fields instead of returning a conflict error. Overrides the CRD-level ForceConflict setting.

---

## Quick reference

| YAML key | Type |
|---|---|
| `name` | string |
| `finalizers` | list |
| `labels` | map |
| `when` | list |
| `reconcile` | boolean |
| `forEach` | object |
| `or` | list |
| `forceConflict` | boolean |
