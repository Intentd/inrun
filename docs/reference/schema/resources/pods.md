# Pod

This declares one Pod to be managed by Inrun.

Prefer DeploymentTemplateSource for long-running workloads. Deployments manage Pod restarts, rolling updates, and replica sets automatically. Use This only when you need direct, single-instance Pod control.

Example:

```yaml
onCreate:
  pods:
    - name: "{{ .metadata.name }}-worker"
      image: "{{ .spec.workerImage }}"
      port: "9090"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — Pod name. Default when omitted: "{{ .metadata.name }}-pod"

---

### `image`

Type: string

Image — container image. Required. Static: "busybox:1.35" or Dynamic: "{{ .spec.image }}"

---

### `imagePullSecrets`

Type: list

ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. If specified, these secrets will be passed to individual puller implementations for them to use.

---

### `port`

Type: string

Port — container port as a string. Static: "8080" or Dynamic: "{{ .spec.port }}"

---

### `protocol`

Type: string

Protocol — network protocol for the container port. Accepted values: TCP (default), UDP, SCTP. Omit to use TCP.

---

### `namespace`

Type: string

Namespace — target namespace. Default when omitted: "{{ .metadata.namespace }}"

---

### `labels`

Type: map

Labels — applied to Pod metadata. Values support template expressions.

---

### `annotations`

Type: map

Annotations — applied to Pod metadata. Values support template expressions.

---

### `resources`

Type: object

Resources — CPU and memory requests/limits for the primary container. Set resources.preset for a named preset, or resources.requests/limits for explicit values. Preset and explicit values are mutually exclusive.

```yaml
resources:
  preset: burst
```

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

---

### `nodeSelector`

Type: map

NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. More info: [https://kubernetes.io/docs/concepts/configuration/assign-pod-node/](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/)

---

### `serviceAccountName`

Type: string

ServiceAccountName is the name of the ServiceAccount to use to run this pod. More info: [https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/)

---

### `restartPolicy`

Type: string

RestartPolicy — Always (the Kubernetes default), OnFailure or Never. Supports template expressions.

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

### `probes`

Type: object

Probes — startup, liveness, and readiness probe configuration.

```yaml
probes:
  liveness:
    type: http
    path: /healthz
    preset: standard
```

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile, not just on create. Equivalent to declaring the same entry under onReconcile. When false (default), only runs on onCreate (idempotent create).

---

### `securityContext`

Type: object

SecurityContext — container-level security settings. Set securityContext.preset for a named preset (baseline, restricted, hardened) or declare individual fields. Preset and explicit fields are mutually exclusive.

```yaml
securityContext:
  preset: restricted
```

---

### `podSecurity`

Type: object

PodSecurity — pod-level security settings applied to the pod spec. Set podSecurity.preset for a named preset or declare individual fields.

```yaml
podSecurity:
  preset: baseline
```

---

### `volumes`

Type: list

Volumes — pod volumes available for mounting into the container.

```yaml
volumes:
  - name: config
    configMap:
      name: myapp-config
```

---

### `volumeMounts`

Type: list

VolumeMounts — mounts for the primary container.

```yaml
volumeMounts:
  - name: config
    mountPath: /etc/myapp
```

---

### `containers`

Type: list

Containers — the pod's containers, when it runs more than one. With containers set, image, command, args, port, probes and resources move into each container, and this entry's env, envFrom, securityContext and volumeMounts become defaults every container inherits. See Container.

---

### `initContainers`

Type: list

InitContainers — run in order, each to completion, before the containers start. They inherit the same defaults; probes are not allowed. See Container.

---

### `forceConflict`

Type: boolean

ForceConflict, when true, sets Force: true when applying this resource, taking ownership of conflicting fields instead of returning a conflict error. Overrides the CRD-level ForceConflict setting.

---

## Quick reference

| YAML key | Type |
|---|---|
| `name` | string |
| `image` | string |
| `imagePullSecrets` | list |
| `port` | string |
| `protocol` | string |
| `namespace` | string |
| `labels` | map |
| `annotations` | map |
| `resources` | object |
| `nodeSelector` | map |
| `serviceAccountName` | string |
| `restartPolicy` | string |
| `when` | list |
| `forEach` | object |
| `or` | list |
| `probes` | object |
| `reconcile` | boolean |
| `securityContext` | object |
| `podSecurity` | object |
| `volumes` | list |
| `volumeMounts` | list |
| `containers` | list |
| `initContainers` | list |
| `forceConflict` | boolean |
