# ConfigMap

This declares one ConfigMap to be managed by Inrun.

ConfigMap data values are static — template expressions are not evaluated in ConfigMap data entries. For dynamic configuration, use a custom Go hook.

Example:

```yaml
onCreate:
  configMaps:
    - name: "{{ .metadata.name }}-config"
      data:
        LOG_LEVEL: info
        MAX_CONNECTIONS: "100"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — ConfigMap name. Default when omitted: "{{ .metadata.name }}-config"

---

### `namespace`

Type: string

Namespace — target namespace. Default when omitted: "{{ .metadata.namespace }}"

---

### `toNamespaces`

Type: list

ToNamespaces - a list of target namespaces Default when omitted: "{{ .metadata.namespace }}"

---

### `data`

Type: map

Data — static key-value configuration entries. Values are plain strings — template expressions are not supported here.

---

### `labels`

Type: map

Labels — applied to ConfigMap metadata. Values support template expressions.

---

### `fromConfigMap`

Type: string

FromConfigMap — name of an existing ConfigMap to copy data from. Inrun reads this at reconcile time — copies stay in sync with the source.

---

### `fromNamespace`

Type: string

FromNamespace — namespace where FromConfigMap lives. Default: same namespace as the CR.

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile. Equivalent to declaring the same entry under both onCreate and onReconcile. When false (default), only runs on onCreate (idempotent create).

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

---

### `forEach`

Type: object

ForEach declares dynamic expansion over a list field. When set, one source declaration becomes N declarations — one per list element. .item and .\<as> are available in template expressions within this declaration.

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
| `namespace` | string |
| `toNamespaces` | list |
| `data` | map |
| `labels` | map |
| `fromConfigMap` | string |
| `fromNamespace` | string |
| `reconcile` | boolean |
| `when` | list |
| `forEach` | object |
| `or` | list |
| `forceConflict` | boolean |
