# Ingress

This declares one Ingress to be managed by Inrun.

Example:

```yaml
onReconcile:
  ingresses:
    - name: "{{ .metadata.name }}-ingress"
      host: "{{ .spec.hostname }}"
      serviceName: "{{ .metadata.name }}-svc"
      servicePort: "{{ .spec.port }}"
      path: /
      pathType: Prefix
      className: nginx
      tls:
        create: true
        secretName: "{{ .metadata.name }}-tls"
        hosts:
          - "{{ .spec.hostname }}"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — Ingress resource name. Default: "{{ .metadata.name }}-ingress"

---

### `namespace`

Type: string

Namespace — target namespace. Default: CR namespace.

---

### `host`

Type: string

Host — virtual host name for the Ingress rule.

---

### `serviceName`

Type: string

ServiceName — backend Service name this Ingress routes to.

---

### `servicePort`

Type: string

ServicePort — backend Service port as a string. Supports template expressions.

---

### `path`

Type: string

Path — HTTP path prefix. Default: "/"

---

### `pathType`

Type: string

PathType — Kubernetes IngressPathType: Prefix, Exact, ImplementationSpecific. Default: Prefix.

---

### `className`

Type: string

IngressClass — Ingress class name (nginx, traefik, etc.). Optional.

---

### `labels`

Type: map

Labels applied to Ingress metadata. Values support template expressions.

---

### `annotations`

Type: map

Annotations applied to Ingress metadata. Values support template expressions.

---

### `tls`

Type: object

TLS — optional TLS configuration. When tls.create is true, Inrun generates a self-signed TLS Secret before creating the Ingress.

```yaml
tls:
  create: true
  secretName: "{{ .metadata.name }}-tls"
  hosts:
    - "{{ .spec.hostname }}"
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
| `namespace` | string |
| `host` | string |
| `serviceName` | string |
| `servicePort` | string |
| `path` | string |
| `pathType` | string |
| `className` | string |
| `labels` | map |
| `annotations` | map |
| `tls` | object |
| `reconcile` | boolean |
| `when` | list |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
