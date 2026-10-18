# Pods

Answer questions about a set of Pods, as the API returns them, passed as a list or a PodList: names, IPs, phases, nodes, readiness, restarts, and why containers are waiting or failed (CrashLoopBackOff, OOMKilled, ImagePullBackOff, …).

## Reference

| Function | Description |
|----------|-------------|
| [`hasCrashingPod`](#hascrashingpod) | Reports whether any pod is unhealthy: restarted more than twice, or a container waiting or terminated with a failure reason (CrashLoopBackOff, OOMKilled, ImagePullBackOff, …). |
| [`podByOrdinal`](#podbyordinal) | Returns the pod whose name ends in -&lt;ordinal&gt;, as StatefulSet pods are named (web-0, web-1, …); nil when none does. |
| [`podContainerReasons`](#podcontainerreasons) | Returns the distinct reasons containers are waiting or terminated with, across all pods, in first-seen order. |
| [`podContainerState`](#podcontainerstate) | Returns the state (Running, Waiting, Terminated) of a named container in the pod at ordinal; "" when either is not found. |
| [`podCount`](#podcount) | Returns the number of pods. |
| [`podCrashLoopDetected`](#podcrashloopdetected) | Reports whether any container in the pods is waiting or terminated with reason CrashLoopBackOff. |
| [`podCreateContainerErrorDetected`](#podcreatecontainererrordetected) | Reports whether any container in the pods is waiting or terminated with reason CreateContainerError. |
| [`podErrImagePullDetected`](#poderrimagepulldetected) | Reports whether any container in the pods is waiting or terminated with reason ErrImagePull. |
| [`podErrorDetected`](#poderrordetected) | Reports whether any container in the pods is waiting or terminated with reason Error. |
| [`podIPs`](#podips) | Returns the pods' IP addresses; pods without one yet are skipped. |
| [`podImagePullBackOffDetected`](#podimagepullbackoffdetected) | Reports whether any container in the pods is waiting or terminated with reason ImagePullBackOff. |
| [`podInvalidImageNameDetected`](#podinvalidimagenamedetected) | Reports whether any container in the pods is waiting or terminated with reason InvalidImageName. |
| [`podMaxRestarts`](#podmaxrestarts) | Returns the highest restart count of any pod (the sum over its containers). |
| [`podNames`](#podnames) | Returns the pods' names, in order. |
| [`podNodes`](#podnodes) | Returns the nodes the pods are scheduled on; pending pods are skipped. |
| [`podOOMKilledDetected`](#podoomkilleddetected) | Reports whether any container in the pods is waiting or terminated with reason OOMKilled. |
| [`podPhases`](#podphases) | Returns each pod's phase, in order. |
| [`podPostStartHookErrorDetected`](#podpoststarthookerrordetected) | Reports whether any container in the pods is waiting or terminated with reason PostStartHookError. |
| [`podPreStartHookErrorDetected`](#podprestarthookerrordetected) | Reports whether any container in the pods is waiting or terminated with reason PreStartHookError. |
| [`podRunContainerErrorDetected`](#podruncontainererrordetected) | Reports whether any container in the pods is waiting or terminated with reason RunContainerError. |
| [`readyPodCount`](#readypodcount) | Returns the number of pods whose Ready condition is True. |

## Functions

### hasCrashingPod

`hasCrashingPod` reports whether any pod is unhealthy: restarted more than twice, or a container waiting or terminated with a failure reason (CrashLoopBackOff, OOMKilled, ImagePullBackOff, …).

```text
{{ hasCrashingPod .pods }}  → true
```

### podByOrdinal

`podByOrdinal` returns the pod whose name ends in -&lt;ordinal&gt;, as StatefulSet pods are named (web-0, web-1, …); nil when none does.

```text
{{ (podByOrdinal .pods 0).status.podIP }}  → "10.0.0.1"
```

### podContainerReasons

`podContainerReasons` returns the distinct reasons containers are waiting or terminated with, across all pods, in first-seen order.

```text
{{ podContainerReasons .pods }}  → [ImagePullBackOff]
```

### podContainerState

`podContainerState` returns the state (Running, Waiting, Terminated) of a named container in the pod at ordinal; "" when either is not found.

```text
{{ podContainerState .pods 0 "app" }}  → "Running"
```

### podCount

`podCount` returns the number of pods.

```text
{{ podCount .pods }}  → 3
```

### podCrashLoopDetected

`podCrashLoopDetected` reports whether any container in the pods is waiting or terminated with reason CrashLoopBackOff.

```text
{{ podCrashLoopDetected .pods }}  → false
```

### podCreateContainerErrorDetected

`podCreateContainerErrorDetected` reports whether any container in the pods is waiting or terminated with reason CreateContainerError.

```text
{{ podCreateContainerErrorDetected .pods }}  → false
```

### podErrImagePullDetected

`podErrImagePullDetected` reports whether any container in the pods is waiting or terminated with reason ErrImagePull.

```text
{{ podErrImagePullDetected .pods }}  → false
```

### podErrorDetected

`podErrorDetected` reports whether any container in the pods is waiting or terminated with reason Error.

```text
{{ podErrorDetected .pods }}  → false
```

### podIPs

`podIPs` returns the pods' IP addresses; pods without one yet are skipped.

```text
{{ podIPs .pods }}  → [10.0.0.1 10.0.0.2]
```

### podImagePullBackOffDetected

`podImagePullBackOffDetected` reports whether any container in the pods is waiting or terminated with reason ImagePullBackOff.

```text
{{ podImagePullBackOffDetected .pods }}  → true
```

### podInvalidImageNameDetected

`podInvalidImageNameDetected` reports whether any container in the pods is waiting or terminated with reason InvalidImageName.

```text
{{ podInvalidImageNameDetected .pods }}  → false
```

### podMaxRestarts

`podMaxRestarts` returns the highest restart count of any pod (the sum over its containers). 0 when there are no pods.

```text
{{ podMaxRestarts .pods }}  → 1
```

### podNames

`podNames` returns the pods' names, in order.

```text
{{ podNames .pods }}  → [web-0 web-1 web-2]
{{ join (podNames .pods) ", " }}  → "web-0, web-1, web-2"
```

### podNodes

`podNodes` returns the nodes the pods are scheduled on; pending pods are skipped.

```text
{{ podNodes .pods }}  → [node-1 node-2]
```

### podOOMKilledDetected

`podOOMKilledDetected` reports whether any container in the pods is waiting or terminated with reason OOMKilled.

```text
{{ podOOMKilledDetected .pods }}  → false
```

### podPhases

`podPhases` returns each pod's phase, in order.

```text
{{ podPhases .pods }}  → [Running Running Pending]
```

### podPostStartHookErrorDetected

`podPostStartHookErrorDetected` reports whether any container in the pods is waiting or terminated with reason PostStartHookError.

```text
{{ podPostStartHookErrorDetected .pods }}  → false
```

### podPreStartHookErrorDetected

`podPreStartHookErrorDetected` reports whether any container in the pods is waiting or terminated with reason PreStartHookError.

```text
{{ podPreStartHookErrorDetected .pods }}  → false
```

### podRunContainerErrorDetected

`podRunContainerErrorDetected` reports whether any container in the pods is waiting or terminated with reason RunContainerError.

```text
{{ podRunContainerErrorDetected .pods }}  → false
```

### readyPodCount

`readyPodCount` returns the number of pods whose Ready condition is True.

```text
{{ readyPodCount .pods }}  → 2
```
