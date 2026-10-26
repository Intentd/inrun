# NetworkPolicy

This declares one NetworkPolicy to be managed by Inrun.

Usage patterns:

1. Inline spec — deny-all ingress:

```yaml
onCreate:
  networkPolicies:
    - name: "{{ .metadata.name }}-deny-all"
      podSelector: {}
      ingress: []
      reconcile: true
```

2. Allow same-namespace ingress:

```yaml
onCreate:
  networkPolicies:
    - name: "{{ .metadata.name }}-allow-same-ns"
      podSelector: {}
      ingress:
        - from:
            - podSelector: {}
```

3. Copy from existing NetworkPolicy:

```yaml
onCreate:
  networkPolicies:
    - name: baseline-policy
      fromNetworkPolicy: org-baseline-policy
      fromNamespace: platform
```

4. Copy to multiple namespaces:

```yaml
onCreate:
  networkPolicies:
    - name: deny-all
      fromNetworkPolicy: org-deny-all
      fromNamespace: platform
      toNamespaces:
        - "{{ .metadata.namespace }}"
        - staging
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — NetworkPolicy name.

---

### `namespace`

Type: string

Namespace — target namespace. Default: "{{ .metadata.namespace }}"

---

### `toNamespaces`

Type: list

ToNamespaces — create one copy in each listed namespace. Each element supports template expressions.

---

### `fromNetworkPolicy`

Type: string

FromNetworkPolicy — name of an existing NetworkPolicy to copy spec from. When set, Inrun reads this NetworkPolicy at reconcile time and copies its spec.

---

### `fromNamespace`

Type: string

FromNamespace — namespace where FromNetworkPolicy lives. Default: same namespace as the CR.

---

### `podSelector`

Type: map

PodSelector — selects the pods this policy applies to. Empty map ({}) selects all pods in the namespace. Values support template expressions.

---

### `ingress`

Type: list

Ingress — list of ingress rules. Empty slice denies all ingress traffic.

---

### `egress`

Type: list

Egress — list of egress rules. Omit to leave egress unmanaged by this policy.

---

### `policyTypes`

Type: list

PolicyTypes — which policy types to enforce. Auto-derived when empty: "Ingress" added when Ingress field is present; "Egress" added when Egress is present.

---

### `labels`

Type: map

Labels — applied to NetworkPolicy metadata.

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

---

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `preset`

Type: string

Preset — named NetworkPolicy preset. Expands into ingress/egress rules and policy types, and the pod selector when podSelector is not set. Allowed values: deny-all, deny-all-ingress, deny-all-egress, allow-same-namespace, allow-dns-egress. Mutually exclusive with Ingress/Egress/PolicyTypes — set preset or explicit rules, not both.

---

### `reconcile`

Type: boolean

Reconcile: true — sync on every reconcile (drift correction).

---

### `forEach`

Type: object

ForEach — expand this entry once per item in a list or map field.

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
| `fromNetworkPolicy` | string |
| `fromNamespace` | string |
| `podSelector` | map |
| `ingress` | list |
| `egress` | list |
| `policyTypes` | list |
| `labels` | map |
| `when` | list |
| `or` | list |
| `preset` | string |
| `reconcile` | boolean |
| `forEach` | object |
| `forceConflict` | boolean |
