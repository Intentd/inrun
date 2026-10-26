# Secret

This declares one Secret to be managed by Inrun.

Secret data values are static — template expressions are not evaluated in Secret data entries. For dynamic configuration, use a custom Go hook.

Example:

```yaml
onCreate:
  secrets:
    - name: "{{ .metadata.name }}-credentials"
      type: Opaque
      data:
        USERNAME: admin
        PASSWORD: "supersecret"
```

You may also copy from an existing Secret using FromSecret.

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — Secret name. Default when omitted: "{{ .metadata.name }}-secret"

---

### `namespace`

Type: string

Namespace — target namespace. Default when omitted: "{{ .metadata.namespace }}"

---

### `type`

Type: string

Type — Kubernetes Secret type. Default: "Opaque"

---

### `data`

Type: map

Data — static key-value entries. Values are plain strings — template expressions are not supported here. If you need templated or dynamic values, use a custom Go hook.

---

### `labels`

Type: map

Labels — applied to Secret metadata. Values support template expressions.

---

### `annotations`

Type: map

Annotations — applied to Secret metadata.

---

### `fromSecret`

Type: string

FromSecret — name of an existing Secret to copy data from. Inrun reads this at reconcile time — copies stay in sync with the source.

---

### `fromNamespace`

Type: string

FromNamespace — namespace where FromSecret lives. Default: same namespace as the CR.

---

### `toNamespaces`

Type: list

ToNamespaces - a list of target namespaces Default when omitted: "{{ .metadata.namespace }}"

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

ForEach declares dynamic expansion (same as other resource types)

---

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `once`

Type: boolean

Once — when true, evaluates templates and creates the Secret only if it does not already exist; every subsequent reconcile is a no-op. Use this for generated credentials that must stay stable across reconciles, in combination with random notes such as randomAlphanumeric, randomHex, and randomBase64. Has no effect when reconcile: true is also set on this entry, or when the entry is declared under onReconcile — continuous reconciliation takes precedence over once. Default: false (standard create/update behavior).

```yaml
secrets:
  - name: "{{ .metadata.name }}-credentials"
    once: true
    data:
      password: "{{ randomAlphanumeric 32 }}"
```

---

### `rotateAfter`

Type: string

RotateAfter declares a time-based rotation threshold. When set alongside once: true, the Secret is recreated when its age exceeds this duration. The creation time is tracked via the annotation:

```yaml
inrun.dev/generated-at: "2026-04-06T08:00:00Z"
```

Supported formats: 30s, 5m, 12h, 90d, 1y Days (d) and years (y) are extensions beyond Go's standard duration format.

Example:

```yaml
secrets:
  - name: "{{ .metadata.name }}-credentials"
    once: true
    rotateAfter: 90d
    data:
      password: "{{ randomAlphanumeric 32 }}"
```

---

### `tls`

Type: object

TLS declares self-signed CA and server certificate generation. When set, the data: block is ignored — the Secret is created as type kubernetes.io/tls with fields: tls.crt, tls.key, ca.crt

Default Secret name when name is empty: "inrun-tls" Default validFor when empty: same as rotateAfter, or "1y"

Example:

```yaml
secrets:
  - name: "{{ .metadata.name }}-tls"
    once: true
    rotateAfter: 1y
    tls:
      commonName: "{{ .metadata.name }}.{{ .metadata.namespace }}.svc"
      dnsNames:
        - "{{ .metadata.name }}"
        - "{{ .metadata.name }}.{{ .metadata.namespace }}"
        - "{{ .metadata.name }}.{{ .metadata.namespace }}.svc"
        - "{{ .metadata.name }}.{{ .metadata.namespace }}.svc.cluster.local"
      validFor: 1y
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
| `type` | string |
| `data` | map |
| `labels` | map |
| `annotations` | map |
| `fromSecret` | string |
| `fromNamespace` | string |
| `toNamespaces` | list |
| `reconcile` | boolean |
| `when` | list |
| `forEach` | object |
| `or` | list |
| `once` | boolean |
| `rotateAfter` | string |
| `tls` | object |
| `forceConflict` | boolean |
