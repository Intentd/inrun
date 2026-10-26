# Custom Resource

This declares one arbitrary Kubernetes resource — any Kind not covered by one of Inrun's built-in resource types (a third-party CRD such as cert-manager's Certificate, or your own CRD). It is schema-agnostic: the spec block is written and applied exactly as declared in the Blueprint, with no built-in knowledge of its structure. Kubernetes enforces the resource's own CRD schema at apply time.

Example:

```yaml
onCreate:
  custom:
    - apiVersion: "cert-manager.io/v1"
      kind: Certificate
      metadata:
        name: "{{ .metadata.name }}-tls"
        namespace: "{{ .metadata.namespace }}"
      spec:
        secretName: "{{ .metadata.name }}-tls-secret"
        dnsNames:
          - "{{ .spec.hostname }}"
        issuerRef:
          name: letsencrypt-prod
          kind: ClusterIssuer
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `apiVersion`

Type: string

APIVersion — the group/version of the target resource, e.g. "cert-manager.io/v1". Required.

---

### `kind`

Type: string

Kind — the target resource's Kind, e.g. "Certificate". Required.

---

### `metadata`

Type: object

Metadata — name, namespace, labels, and annotations for the resource. name is required. namespace is required for namespaced CRDs and must be omitted for cluster-scoped ones; Inrun determines the correct scope via discovery unless namespaced is set explicitly.

```yaml
metadata:
  name: "{{ .metadata.name }}-tls"
  namespace: "{{ .metadata.namespace }}"
  labels:
    app: "{{ .metadata.name }}"
```

---

### `spec`

Type: map

Spec is the conventional spec block for CRDs. It is schema-agnostic and may contain templated values. Only template syntax is validated by Inrun; structural/schema validation is deferred to the API server.

---

### `status`

Type: map

Status — an initial status block, useful when bootstrapping a resource that expects one to be present immediately. Inrun only writes this if hasStatus resolves to true. Prefer letting the resource's own controller populate status rather than setting this.

---

### `hasStatus`

Type: boolean

HasStatus is an explicit hint about whether the CRD exposes a status subresource. Three states are useful:

  - nil: auto-detect via discovery at runtime
  - true: force status writes (patches)
  - false: never attempt status writes

Use this to avoid API errors for CRDs that do not support status.

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
| `apiVersion` | string |
| `kind` | string |
| `metadata` | object |
| `spec` | map |
| `status` | map |
| `hasStatus` | boolean |
| `reconcile` | boolean |
| `when` | list |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
