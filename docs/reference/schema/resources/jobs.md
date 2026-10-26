# Job

This declares one Job to be run by Inrun.

Most commonly used under onDelete for cleanup tasks that must complete before Inrun removes finalizers from the CR:

  - Draining queues or buffers
  - Archiving state to external storage
  - Notifying external systems of deletion
  - Running database migrations before removing a CRD instance

Can also be used under onCreate for one-time provisioning tasks.

Example (cleanup on delete):

```yaml
onDelete:
  jobs:
    - name: "{{ .metadata.name }}-cleanup"
      image: busybox
      command: ["sh", "-c", "echo cleaning up {{ .metadata.name }}"]
      backoffLimit: 3
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Job entries are always a one-time create — `reconcile: true` and `onReconcile` have no effect. Job is also commonly declared under `onDelete`, for cleanup work that must complete before the CR's finalizer is removed.

---

## Fields

### `name`

Type: string

Name — Job name. Default when omitted: "{{ .metadata.name }}-job"

---

### `image`

Type: string

Image — container image. Required.

---

### `imagePullSecrets`

Type: list

ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. If specified, these secrets will be passed to individual puller implementations for them to use.

---

### `command`

Type: list

Command — container entrypoint command. Each element is resolved independently — template expressions are supported per element. e.g. \["sh", "-c", "echo cleaning up {{ .metadata.name }}"]

---

### `args`

Type: list

Args — arguments passed to the container command. Each element supports template expressions independently.

---

### `backoffLimit`

Type: number

BackoffLimit — number of Pod restart attempts before the Job is marked Failed. Default: 3.

---

### `namespace`

Type: string

Namespace — target namespace. Default when omitted: "{{ .metadata.namespace }}"

---

### `labels`

Type: map

Labels — applied to Job metadata. Values support template expressions.

---

### `nodeSelector`

Type: map

NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. More info: [https://kubernetes.io/docs/concepts/configuration/assign-pod-node/](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) +optional +mapType=atomic

---

### `serviceAccountName`

Type: string

ServiceAccountName is the name of the ServiceAccount to use to run this pod. More info: [https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) +optional

---

### `restartPolicy`

Type: string

RestartPolicy — OnFailure (default) or Never. With Never, each retry starts a new pod and the failed pods are kept, logs included. Supports template expressions.

---

### `when`

Type: list

When lists conditions that must all pass for this Job to be created; see Condition. A Job is not removed when they stop passing: it runs to completion.

---

### `reconcile`

Type: boolean

Reconcile has no effect on a Job entry. Jobs are always a one-time, idempotent create — Inrun never re-applies or updates a Job after it runs, since Jobs are meant to run once to completion. This field exists for schema consistency with other resource types only.

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

### `resources`

Type: object

Resources — CPU and memory requests/limits for the container. Set resources.preset for a named preset, or resources.requests/limits for explicit values. Preset and explicit values are mutually exclusive.

```yaml
resources:
  preset: burst
```

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
| `command` | list |
| `args` | list |
| `backoffLimit` | number |
| `namespace` | string |
| `labels` | map |
| `nodeSelector` | map |
| `serviceAccountName` | string |
| `restartPolicy` | string |
| `when` | list |
| `reconcile` | boolean |
| `forEach` | object |
| `or` | list |
| `resources` | object |
| `securityContext` | object |
| `podSecurity` | object |
| `volumes` | list |
| `volumeMounts` | list |
| `containers` | list |
| `initContainers` | list |
| `forceConflict` | boolean |
