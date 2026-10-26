# ReplicaSet

This declares one ReplicaSet to be managed by Inrun.

Minimal example — static values only:

```yaml
onCreate:
  replicasets:
    - image: nginx:1.25
      replicas: "3"
      port: "8080"
```

Full example — dynamic values from the CR:

```yaml
onCreate:
  replicasets:
    - name: "{{ .metadata.name }}-app"
      image: "{{ .spec.image }}"
      replicas: "{{ .spec.replicas }}"
      port: "{{ .spec.port }}"
      namespace: "{{ .metadata.namespace }}"
      labels:
        - key: app
          value: "{{ .metadata.name }}"
        - key: managed-by
          value: inrun
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 500m
          memory: 512Mi
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — ReplicaSet and primary container name. Supports template expressions. Default when omitted: "{{ .metadata.name }}-replicaset"

---

### `image`

Type: string

Image — container image. Required (must be declared here or resolvable from CR). Static:  "nginx:1.25" Dynamic: "{{ .spec.image }}"

---

### `imagePullSecrets`

Type: list

ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec. If specified, these secrets will be passed to individual puller implementations for them to use.

---

### `replicas`

Type: string

Replicas — number of pod replicas as a string. Static:  "3" Dynamic: "{{ .spec.replicas }}" Default: "1"

---

### `port`

Type: string

Port — primary container port as a string. Static:  "8080" Dynamic: "{{ .spec.port }}" Omit to expose no port.

---

### `protocol`

Type: string

Protocol — network protocol for the container port. Accepted values: TCP (default), UDP, SCTP. Omit to use TCP.

---

### `namespace`

Type: string

Namespace — target namespace for the ReplicaSet. Default when omitted: "{{ .metadata.namespace }}" (same namespace as the CR).

---

### `labels`

Type: map

Labels — applied to the ReplicaSet ObjectMeta and the pod template. Label values support template expressions. Inrun always adds: managed-by=inrun, inrun-owner=\<cr-name>

---

### `annotations`

Type: map

Annotations — applied to the ReplicaSet ObjectMeta only. Annotation values support template expressions.

---

### `resources`

Type: object

Resources — CPU and memory requests/limits for the primary container. Set resources.preset for a named preset, or resources.requests/limits for explicit values. Preset and explicit values are mutually exclusive.

```yaml
resources:
  preset: burst
```

---

### `env`

Type: list

Env — environment variables for the primary container, in Kubernetes-native list format. Each entry has a name and either a value or a valueFrom source. If omitted, no environment variables are added.

```yaml
env:
  - name: LOG_LEVEL
    value: info
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: myapp-secrets
        key: api-key
```

---

### `envFrom`

Type: object

EnvFrom — bulk-load environment variables from Secrets and/or ConfigMaps into the primary container, in addition to any individual entries in env. Each secretRef/configMapRef entry names an existing Secret or ConfigMap; every key in it becomes an environment variable.

```yaml
envFrom:
  secretRef:
    - name: myapp-secrets
  configMapRef:
    - name: myapp-config
```

---

### `nodeSelector`

Type: map

NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. More info: [https://kubernetes.io/docs/concepts/configuration/assign-pod-node/](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) +optional +mapType=atomic

---

### `serviceAccountName`

Type: string

ServiceAccountName is the name of the ServiceAccount to use to run this pod. More info: [https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) +optional

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

ForEach declares dynamic expansion over a list field.

---

### `autoscale`

Type: object

Autoscale declares workload autoscaling behaviour for this ReplicaSet.

---

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `probes`

Type: object

Probes — startup, liveness, and readiness probe configuration.

---

### `securityContext`

Type: object

SecurityContext — container-level security settings. Set securityContext.preset for a named preset (baseline, restricted, hardened) or declare individual fields. Preset and explicit fields are mutually exclusive.

---

### `podSecurity`

Type: object

PodSecurity — pod-level security settings applied to the pod spec. Set podSecurity.preset for a named preset or declare individual fields.

---

### `volumes`

Type: list

Volumes — pod volumes available for mounting into the container.

---

### `volumeMounts`

Type: list

VolumeMounts — mounts for the primary container.

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
| `replicas` | string |
| `port` | string |
| `protocol` | string |
| `namespace` | string |
| `labels` | map |
| `annotations` | map |
| `resources` | object |
| `env` | list |
| `envFrom` | object |
| `nodeSelector` | map |
| `serviceAccountName` | string |
| `reconcile` | boolean |
| `when` | list |
| `forEach` | object |
| `autoscale` | object |
| `or` | list |
| `probes` | object |
| `securityContext` | object |
| `podSecurity` | object |
| `volumes` | list |
| `volumeMounts` | list |
| `containers` | list |
| `initContainers` | list |
| `forceConflict` | boolean |
