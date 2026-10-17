# 17 — Pod Notes

Pod notes navigate the `_pods` enrichment embedded by `children.go` into Deployment, StatefulSet, and ReplicaSet resource maps. They let templates surface pod names, IPs, counts, and health without hooks or custom Go code.

Pod enrichment is opt-in at the CRD level:

```yaml
spec:
  crds:
    my-operator:
      enrich: [pods]   # or enrichAll: true
      operatorBox:
        ...
```

Without enrichment, `_pods` is absent and all pod notes return their zero values (`""`, `0`, `false`) — safe to use in templates regardless.

---

## Reference

### `podNames`

Return a comma-separated string of pod names owned by the enriched resource.

Keywords: pods, names, list, enriched, deployment, statefulset, string

```yaml
status:
  fields:
    - path: pods
      value: "{{ podNames .children.deployment }}"
# → "web-abc, web-def"
```

---

### `podIPs`

Return a comma-separated string of pod IP addresses. Returns `""` while pods are pending (IPs not yet assigned).

Keywords: pods, ip, list, enriched, address, network, string

```yaml
- path: podIPs
  value: "{{ podIPs .children.statefulset }}"
# → "10.0.0.1, 10.0.0.2, 10.0.0.3"
```

---

### `podPhases`

Return a comma-separated string of pod phases in order. Useful for surfacing the phase distribution of a multi-replica Deployment or StatefulSet.

Keywords: pods, phase, list, enriched, status, lifecycle, string

```yaml
- path: podPhases
  value: "{{ podPhases .children.statefulset }}"
# → "Running, Running, Pending"
```

---

### `podNodes`

Return a comma-separated list of node names the pods are scheduled on. Returns `""` while pods are Pending (not yet assigned to a node).

Keywords: pods, node, list, enriched, scheduled, placement, string

```yaml
- path: podNodes
  value: "{{ podNodes .children.deployment }}"
# → "node-1, node-2"
```

---

### `podCount`

Return the total number of pods as `int`.

Keywords: pods, count, int, enriched, total, size

```yaml
- path: podCount
  value: "{{ podCount .children.deployment }}"
# → 3
```

---

### `readyPodCount`

Return the number of pods whose `Ready` condition is `True`.

Keywords: pods, ready, count, int, enriched, health

```yaml
- path: readyPods
  value: "{{ readyPodCount .children.deployment }}"
# → 2
```

---

### `podMaxRestarts`

Return the highest restart count across all pods as `int64`. Zero when no pods are present or none have restarted. Pairs with `hasCrashingPod` for full crash-loop visibility.

Keywords: pods, restart, count, crash, enriched, int, max

```yaml
- path: maxRestarts
  value: "{{ podMaxRestarts .children.deployment }}"
# → 3
```

---

### `hasCrashingPod`

Return `true` when any pod has restarted more than twice — the first declarative signal of a crash loop.

Keywords: pods, crash, restart, boolean, enriched, loop, health

```yaml
when:
  - field: "{{ hasCrashingPod .children.deployment }}"
    equals: "false"

status:
  fields:
    - path: crashDetected
      value: "{{ hasCrashingPod .children.deployment }}"
    - path: maxRestarts
      value: "{{ podMaxRestarts .children.deployment }}"
```

---

### `podByOrdinal`

Return the pod summary map at the given ordinal index. Designed for StatefulSets whose pods are named `<name>-0`, `<name>-1`, etc. Returns `nil` when no pod with that ordinal exists.

Keywords: pods, statefulset, ordinal, index, enriched, primary, member

```yaml
# Surface the primary StatefulSet member:
- path: primaryPod
  value: "{{ (podByOrdinal .children.statefulset 0).name }}"
- path: primaryIP
  value: "{{ (podByOrdinal .children.statefulset 0).ip }}"

# All fields available on the returned map:
# .name, .ip, .phase, .ready, .node, .restartCount, .ordinal
```

---

### `podCrashLoopDetected`

Return `true` when any container across any pod is in `CrashLoopBackOff`. More precise than `hasCrashingPod`, which only checks restart count.

Keywords: pods, crash, loop, boolean, enriched, crashloopbackoff, health, container

```yaml
- path: crashLoop
  value: "{{ podCrashLoopDetected .children.deployment }}"
```

---

### `podImagePullBackOffDetected`

Return `true` when any container across any pod has reason `ImagePullBackOff` (image could not be pulled due to authentication, network, or missing image).

Keywords: pods, image, pull, error, boolean, enriched, imagepullbackoff, registry

```yaml
- path: imagePullBackOff
  value: "{{ podImagePullBackOffDetected .children.deployment }}"
```

---

### `podErrImagePullDetected`

Return `true` when any container has reason `ErrImagePull` (a transient image pull failure, usually preceding `ImagePullBackOff`).

Keywords: pods, image, pull, error, boolean, enriched, errimagepull, transient

```yaml
- path: errImagePull
  value: "{{ podErrImagePullDetected .children.deployment }}"
```

---

### `podErrorDetected`

Return `true` when any container has reason `Error` (the container process exited with a non‑zero code or was terminated by the system).

Keywords: pods, error, exit, boolean, enriched, container, failed

```yaml
- path: containerError
  value: "{{ podErrorDetected .children.deployment }}"
```

---

### `podOOMKilledDetected`

Return `true` when any container has reason `OOMKilled` (the container was terminated because it exhausted its memory limit).

Keywords: pods, oom, memory, killed, boolean, enriched, oomkilled, limit

```yaml
- path: oomKilled
  value: "{{ podOOMKilledDetected .children.deployment }}"
```

---

### `podRunContainerErrorDetected`

Return `true` when any container across any pod has reason `RunContainerError` (container failed to start, e.g., misconfigured command or missing binary).

Keywords: pods, container, error, start, enriched, runcontainererror, boolean

```yaml
- path: runContainerError
  value: "{{ podRunContainerErrorDetected .children.deployment }}"
```

---

### `podCreateContainerErrorDetected`

Return `true` when any container has reason `CreateContainerError` (container creation failed, typically due to volume mount issues or resource constraints).

Keywords: pods, container, error, create, enriched, createcontainererror, volume, boolean

```yaml
- path: createContainerError
  value: "{{ podCreateContainerErrorDetected .children.deployment }}"
```

---

### `podInvalidImageNameDetected`

Return `true` when any container has reason `InvalidImageName` (the image name could not be parsed by the container runtime).

Keywords: pods, image, name, invalid, enriched, invalidimagename, boolean, registry

```yaml
- path: invalidImageName
  value: "{{ podInvalidImageNameDetected .children.deployment }}"
```

---

### `podPreStartHookErrorDetected`

Return `true` when any container has reason `PreStartHookError` (the pre‑start lifecycle hook failed).

Keywords: pods, hook, lifecycle, error, enriched, prestarthookerror, boolean

```yaml
- path: preStartHookError
  value: "{{ podPreStartHookErrorDetected .children.deployment }}"
```

---

### `podPostStartHookErrorDetected`

Return `true` when any container has reason `PostStartHookError` (the post‑start lifecycle hook failed).

Keywords: pods, hook, lifecycle, error, enriched, poststarthookerror, boolean

```yaml
- path: postStartHookError
  value: "{{ podPostStartHookErrorDetected .children.deployment }}"
```

---

### `podContainerReasons`

Return a comma-separated list of unique waiting or terminated reasons across all containers in all pods. Empty reasons are omitted. Useful for surfacing `ImagePullBackOff`, `CrashLoopBackOff`, `OOMKilled`, etc.

Keywords: pods, container, reasons, error, list, enriched, string, status

```yaml
- path: containerReasons
  value: "{{ podContainerReasons .children.deployment }}"
# → "CrashLoopBackOff, ImagePullBackOff"
```

---

### `podContainerState`

Return the state of a named container within the pod at the given ordinal. Returns `""` when the pod or container is not found.

Keywords: pods, container, state, statefulset, ordinal, enriched, string, running, waiting

```yaml
- path: appContainerState
  value: "{{ podContainerState .children.statefulset 0 \"app\" }}"
# → "Running"
# → "Waiting"
```

---

## Zero-code memcached operator

```yaml
spec:
  crds:
    memcached:
      enrich: [pods]
      operatorBox:
        onCreate:
          deployments:
            - name: "{{ .metadata.name }}"
              image: "memcache:{{ .spec.version }}"
              replicas: "{{ .spec.size }}"

        status:
          fields:
            - path: pods
              value: "{{ podNames .children.deployment }}"
            - path: podIPs
              value: "{{ podIPs .children.deployment }}"
            - path: size
              value: "{{ podCount .children.deployment }}"
            - path: ready
              value: "{{ readyPodCount .children.deployment }}"
```

`kubectl get memcached <name> -o yaml` shows `status.pods` populated with running pod names — no hooks, no Go code.

---

## Quick reference

| Note | Signature | Returns |
|------|-----------|---------|
| `podNames` | `(obj any)` | `string` |
| `podIPs` | `(obj any)` | `string` |
| `podPhases` | `(obj any)` | `string` |
| `podNodes` | `(obj any)` | `string` |
| `podCount` | `(obj any)` | `int` |
| `readyPodCount` | `(obj any)` | `int` |
| `podMaxRestarts` | `(obj any)` | `int64` |
| `hasCrashingPod` | `(obj any)` | `bool` |
| `podByOrdinal` | `(obj any, ordinal int64)` | `any` |
| `podCrashLoopDetected` | `(obj any)` | `bool` |
| `podImagePullBackOffDetected` | `(obj any)` | `bool` |
| `podErrImagePullDetected` | `(obj any)` | `bool` |
| `podErrorDetected` | `(obj any)` | `bool` |
| `podOOMKilledDetected` | `(obj any)` | `bool` |
| `podRunContainerErrorDetected` | `(obj any)` | `bool` |
| `podCreateContainerErrorDetected` | `(obj any)` | `bool` |
| `podInvalidImageNameDetected` | `(obj any)` | `bool` |
| `podPreStartHookErrorDetected` | `(obj any)` | `bool` |
| `podPostStartHookErrorDetected` | `(obj any)` | `bool` |
| `podContainerReasons` | `(obj any)` | `string` |
| `podContainerState` | `(obj any, ordinal int64, name string)` | `string` |

Requires `enrich: [pods]` or `enrichAll: true` on the CRD.

Each pod in `_pods` carries: `name`, `ip`, `phase`, `ready`, `node`, `restartCount`, `ordinal` (-1 for non-StatefulSet pods).

---

**Next →** [18 — Semver Notes](18-semver.md)
