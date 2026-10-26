# HorizontalPodAutoscaler

This declares one HorizontalPodAutoscaler to be managed by Inrun.

Example:

```yaml
onReconcile:
  hpa:
    - name: "{{ .metadata.name }}-hpa"
      deploymentRef: "{{ .metadata.name }}"
      minReplicas: "{{ .spec.minReplicas }}"
      maxReplicas: "{{ .spec.maxReplicas }}"
      targetCPUUtilizationPercentage: "80"
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

Name — HPA resource name. Default: "{{ .metadata.name }}-hpa"

---

### `namespace`

Type: string

Namespace — target namespace. Default: CR namespace.

---

### `scaleTargetRef`

Type: object

ScaleTargetRef — the target workload this HPA scales. Supports Deployment, ReplicaSet, StatefulSet, or any scalable resource. Supports template expressions: "{{ .metadata.name }}"

```yaml
scaleTargetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: "{{ .metadata.name }}-deployment"
```

---

### `minReplicas`

Type: string

MinReplicas — minimum replica count as a string. Supports template expressions.

---

### `maxReplicas`

Type: string

MaxReplicas — maximum replica count as a string. Supports template expressions.

---

### `targetCPUUtilizationPercentage`

Type: string

TargetCPUUtilizationPercentage — CPU utilization target (0-100). Supports templates. When behavior.preset is set and this field is empty, the preset provides the default.

---

### `behavior`

Type: object

Behavior — scale-up and scale-down behavior configuration. Set preset for a complete preset, or configure scaleUp/scaleDown explicitly. Preset and explicit fields are mutually exclusive.

```yaml
behavior:
  preset: web
```

```yaml
behavior:
  scaleUp:
    stabilizationWindowSeconds: 0
  scaleDown:
    stabilizationWindowSeconds: 300
```

---

### `labels`

Type: map

Labels applied to HPA metadata. Values support template expressions.

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
| `scaleTargetRef` | object |
| `minReplicas` | string |
| `maxReplicas` | string |
| `targetCPUUtilizationPercentage` | string |
| `behavior` | object |
| `labels` | map |
| `reconcile` | boolean |
| `when` | list |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
