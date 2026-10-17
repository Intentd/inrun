# Pod Notes

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

| Note | Description |
|------|-------------|
| `podNames` | Return a comma-separated string of pod names owned by the enriched resource. |
| `podIPs` | Return a comma-separated string of pod IP addresses. |
| `podPhases` | Return a comma-separated string of pod phases in order. |
| `podNodes` | Return a comma-separated list of node names the pods are scheduled on. |
| `podCount` | Return the total number of pods as `int`. |
| `readyPodCount` | Return the number of pods whose `Ready` condition is `True`. |
| `podMaxRestarts` | Return the highest restart count across all pods as `int64`. |
| `hasCrashingPod` | Return `true` when any pod has restarted more than twice — the first declarative signal of a crash loop. |
| `podByOrdinal` | Return the pod summary map at the given ordinal index. |
| `podCrashLoopDetected` | Return `true` when any container across any pod is in `CrashLoopBackOff`. |
| `podImagePullBackOffDetected` | Return `true` when any container across any pod has reason `ImagePullBackOff` (image could not be pulled due to authentication, network, or missing image). |
| `podErrImagePullDetected` | Return `true` when any container has reason `ErrImagePull` (a transient image pull failure, usually preceding `ImagePullBackOff`). |
| `podErrorDetected` | Return `true` when any container has reason `Error` (the container process exited with a non‑zero code or was terminated by the system). |
| `podOOMKilledDetected` | Return `true` when any container has reason `OOMKilled` (the container was terminated because it exhausted its memory limit). |
| `podRunContainerErrorDetected` | Return `true` when any container across any pod has reason `RunContainerError` (container failed to start, e. |
| `podCreateContainerErrorDetected` | Return `true` when any container has reason `CreateContainerError` (container creation failed, typically due to volume mount issues or resource constraints). |
| `podInvalidImageNameDetected` | Return `true` when any container has reason `InvalidImageName` (the image name could not be parsed by the container runtime). |
| `podPreStartHookErrorDetected` | Return `true` when any container has reason `PreStartHookError` (the pre‑start lifecycle hook failed). |
| `podPostStartHookErrorDetected` | Return `true` when any container has reason `PostStartHookError` (the post‑start lifecycle hook failed). |
| `podContainerReasons` | Return a comma-separated list of unique waiting or terminated reasons across all containers in all pods. |
| `podContainerState` | Return the state of a named container within the pod at the given ordinal. |

## Examples

```yaml
# podNames
status:
  fields:
    - path: pods
      value: "{{ podNames .children.deployment }}"
# → "web-abc, web-def"

# podIPs
- path: podIPs
  value: "{{ podIPs .children.statefulset }}"
# → "10.0.0.1, 10.0.0.2, 10.0.0.3"

# podPhases
- path: podPhases
  value: "{{ podPhases .children.statefulset }}"
# → "Running, Running, Pending"

# podNodes
- path: podNodes
  value: "{{ podNodes .children.deployment }}"
# → "node-1, node-2"

# podCount
- path: podCount
  value: "{{ podCount .children.deployment }}"
# → 3

# readyPodCount
- path: readyPods
  value: "{{ readyPodCount .children.deployment }}"
# → 2

# podMaxRestarts
- path: maxRestarts
  value: "{{ podMaxRestarts .children.deployment }}"
# → 3

# hasCrashingPod
when:
  - field: "{{ hasCrashingPod .children.deployment }}"
    equals: "false"

status:
  fields:
    - path: crashDetected
      value: "{{ hasCrashingPod .children.deployment }}"
    - path: maxRestarts
      value: "{{ podMaxRestarts .children.deployment }}"

# podByOrdinal
# Surface the primary StatefulSet member:
- path: primaryPod
  value: "{{ (podByOrdinal .children.statefulset 0).name }}"
- path: primaryIP
  value: "{{ (podByOrdinal .children.statefulset 0).ip }}"

# All fields available on the returned map:
# .name, .ip, .phase, .ready, .node, .restartCount, .ordinal

# podCrashLoopDetected
- path: crashLoop
  value: "{{ podCrashLoopDetected .children.deployment }}"

# podImagePullBackOffDetected
- path: imagePullBackOff
  value: "{{ podImagePullBackOffDetected .children.deployment }}"

# podErrImagePullDetected
- path: errImagePull
  value: "{{ podErrImagePullDetected .children.deployment }}"

# podErrorDetected
- path: containerError
  value: "{{ podErrorDetected .children.deployment }}"

# podOOMKilledDetected
- path: oomKilled
  value: "{{ podOOMKilledDetected .children.deployment }}"

# podRunContainerErrorDetected
- path: runContainerError
  value: "{{ podRunContainerErrorDetected .children.deployment }}"

# podCreateContainerErrorDetected
- path: createContainerError
  value: "{{ podCreateContainerErrorDetected .children.deployment }}"

# podInvalidImageNameDetected
- path: invalidImageName
  value: "{{ podInvalidImageNameDetected .children.deployment }}"

# podPreStartHookErrorDetected
- path: preStartHookError
  value: "{{ podPreStartHookErrorDetected .children.deployment }}"

# podPostStartHookErrorDetected
- path: postStartHookError
  value: "{{ podPostStartHookErrorDetected .children.deployment }}"

# podContainerReasons
- path: containerReasons
  value: "{{ podContainerReasons .children.deployment }}"
# → "CrashLoopBackOff, ImagePullBackOff"

# podContainerState
- path: appContainerState
  value: "{{ podContainerState .children.statefulset 0 \"app\" }}"
# → "Running"
# → "Waiting"
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
