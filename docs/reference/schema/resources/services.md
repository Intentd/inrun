# Service

This declares one Service to be managed by Inrun.

Example:

```yaml
onCreate:
  services:
    - name: "{{ .metadata.name }}-svc"
      type: ClusterIP
      port: "80"
      targetPort: "8080"
      namespace: "{{ .metadata.namespace }}"
      labels:
        app: "{{ .metadata.name }}"
      selector:
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

Name — Service name. Default when omitted: "{{ .metadata.name }}-svc"

---

### `type`

Type: string

Type — Kubernetes Service type. Accepted values: ClusterIP, NodePort, LoadBalancer. Default: ClusterIP.

---

### `headless`

Type: boolean

Headless — when true, the Service is created without a clusterIP (clusterIP: None). Used primarily for StatefulSets to enable stable network identities and per‑pod DNS:

```yaml
<podname>.<service>.<namespace>.svc.cluster.local
```

Set this to true when the Service is meant to back a StatefulSet or provide direct pod‑to‑pod addressing rather than load‑balanced traffic.

---

### `protocol`

Type: string

Protocol defines network protocols supported for things like container ports. "TCP", "UDP", "SCTP"

---

### `port`

Type: string

Port — Service port as a string. Static: "80" or Dynamic: "{{ .spec.servicePort }}"

---

### `targetPort`

Type: string

TargetPort — container port the Service routes traffic to. Static: "8080" or Dynamic: "{{ .spec.containerPort }}"

---

### `namespace`

Type: string

Namespace — target namespace. Default when omitted: "{{ .metadata.namespace }}"

---

### `labels`

Type: map

Labels — applied to Service metadata. Values support template expressions.

---

### `selector`

Type: map

Selector filters which pods this service will route traffic to. Useful in forEach situations where the labels would likely be the same.

```yaml
selector:
  app: "{{ .metadata.name }}"
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
| `type` | string |
| `headless` | boolean |
| `protocol` | string |
| `port` | string |
| `targetPort` | string |
| `namespace` | string |
| `labels` | map |
| `selector` | map |
| `reconcile` | boolean |
| `when` | list |
| `forEach` | object |
| `or` | list |
| `forceConflict` | boolean |
