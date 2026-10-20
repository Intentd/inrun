# PodDisruptionBudget

This declares one PodDisruptionBudget to be managed by Inrun.

Example:

```yaml
onReconcile:
  pdb:
    - name: "{{ .metadata.name }}-pdb"
      minAvailable: "1"
      selector:
        app: "{{ .metadata.name }}"
      forEach:
        field: spec.services
        as: item
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — PDB resource name. Default: "{{ .metadata.name }}-pdb"

---

### `namespace`

Type: string

Namespace — target namespace. Default: CR namespace.

---

### `selector`

Type: map

Selector — label selector identifying the pods this PDB protects. Keys are static; values support template expressions.

---

### `minAvailable`

Type: string

MinAvailable — minimum number of pods that must remain available. Accepts integer strings ("1") or percentage strings ("50%"). Mutually exclusive with MaxUnavailable.

---

### `maxUnavailable`

Type: string

MaxUnavailable — maximum number of pods that may be unavailable. Accepts integer strings ("1") or percentage strings ("25%"). Mutually exclusive with MinAvailable.

---

### `labels`

Type: map

Labels applied to PDB metadata. Values support template expressions.

---

### `behavior`

Type: object

Behavior — disruption limit configuration. Set behavior.preset for a named preset, or declare minAvailable/maxUnavailable explicitly. Preset and explicit fields are mutually exclusive.

```yaml
behavior:
  preset: zero-downtime
```

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile. Equivalent to declaring the same entry under both onCreate and onReconcile. When false (default), only runs on onCreate (idempotent create).

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

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
  field: spec.services
  as: item
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
| `namespace` | string |
| `selector` | map |
| `minAvailable` | string |
| `maxUnavailable` | string |
| `labels` | map |
| `behavior` | object |
| `reconcile` | boolean |
| `when` | list |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
