# Kubernetes

Answer the questions about any Kubernetes object that every controller asks: its conditions and their reasons, whether it is ready, synced, terminating or exists, and the fields of its spec and status, read safely.

## Reference

| Function | Description |
|----------|-------------|
| [`annotations`](#annotations) | Returns the `metadata.annotations` map. |
| [`conditionMessage`](#conditionmessage) | Returns the `message` field of a named condition. |
| [`conditionReason`](#conditionreason) | Returns the `reason` field of a named condition. |
| [`conditionsHealthy`](#conditionshealthy) | Reports whether an object's standard conditions say it is healthy, for any kind that reports them: Flux, cert-manager, Crossplane, Argo and most operators. |
| [`conditionsState`](#conditionsstate) | Reads an object's standard conditions into one of "healthy", "progressing", "failed" or "unknown", for any kind that reports them: Flux, cert-manager, Crossplane, Argo, Deployments and most operators. |
| [`conditionsStateMessage`](#conditionsstatemessage) | Returns the message of the condition that decided conditionsState: Stalled, Synced, Progressing, Reconciling, Ready or Available. |
| [`expectState`](#expectstate) | Judges one status value the way conditionsState judges conditions, for kinds that report a field instead of standard conditions: "healthy" when the value matches healthy, "failed" when it matches failed, and "progressing" otherwise, including when the value is absent or empty (not reported yet). |
| [`firstState`](#firststate) | Returns the first of its states that says something: the first that is neither "unknown" nor empty, or "unknown" when none does. |
| [`generation`](#generation) | Returns `metadata.generation` as int64. |
| [`get`](#get) | Navigates a nested path through a Kubernetes object using variadic string segments. |
| [`getAnnotation`](#getannotation) | Returns the value of a single annotation key from `metadata.annotations`. |
| [`getAnnotationInt`](#getannotationint) | Returns an annotation value parsed as `int64`. |
| [`getLabel`](#getlabel) | Returns the value of a single label key from `metadata.labels`. |
| [`getLabelInt`](#getlabelint) | Returns a label value parsed as `int64`. |
| [`getStatus`](#getstatus) | Returns the string value of a specific key from `status`. |
| [`hasAnnotation`](#hasannotation) | Returns `true` when the object has the given annotation key with a non-empty value. |
| [`hasCondition`](#hascondition) | Returns `true` if `status.conditions` contains a condition of the given type with `status: "True"`. |
| [`hasLabel`](#haslabel) | Returns `true` when the object has the given label key with a non-empty value. |
| [`hasStatus`](#hasstatus) | Returns `true` when the given status field exists and is non-empty. |
| [`isSynced`](#issynced) | Returns `true` when `metadata.generation == status.observedGeneration`, meaning the controller has fully processed the current spec. |
| [`isTerminating`](#isterminating) | Returns `true` when `metadata.deletionTimestamp` is set — the object exists but is being deleted. |
| [`labelMatches`](#labelmatches) | Returns `true` when the object's labels contain every key/value pair given as arguments. |
| [`labels`](#labels) | Returns the `metadata.labels` map. |
| [`meta`](#meta) | Returns the `metadata` map of a Kubernetes object. |
| [`name`](#name) | Returns metadata.name or an empty string. |
| [`namespace`](#namespace) | Returns metadata.namespace or an empty string. |
| [`observedGeneration`](#observedgeneration) | Returns `status.observedGeneration` as int64. |
| [`ownerKind`](#ownerkind) | Returns the `kind` of the first `ownerReference`. |
| [`ownerName`](#ownername) | Returns the `name` of the first `ownerReference`. |
| [`phase`](#phase) | Returns `status.phase` as a string. |
| [`resourceCPU`](#resourcecpu) | Returns `spec.resources.requests.cpu` from any object. |
| [`resourceExists`](#resourceexists) | Reports whether obj is a Kubernetes object: a map with a metadata.name. |
| [`resourceMemory`](#resourcememory) | Returns `spec.resources.requests.memory` from any object. |
| [`spec`](#spec) | Returns the `spec` map of a Kubernetes object. |
| [`status`](#status) | Returns the `status` map of a Kubernetes object. |

## Functions

### annotations

`annotations` returns the `metadata.annotations` map. Returns an empty map if absent.

```text
{{ mapGet (annotations .deployment) "example.com/owner" }}  → "team-payments"
```

### conditionMessage

`conditionMessage` returns the `message` field of a named condition. Returns `""` when absent.

```text
{{ conditionMessage .deployment "Available" }}  → "Deployment has minimum availability."
```

### conditionReason

`conditionReason` returns the `reason` field of a named condition. Returns `""` when absent.

```text
{{ conditionReason .deployment "Available" }}  → "MinimumReplicasAvailable"
```

### conditionsHealthy

`conditionsHealthy` reports whether an object's standard conditions say it is healthy, for any kind that reports them: Flux, cert-manager, Crossplane, Argo and most operators. It is true when the controller has seen the latest spec and either Ready, or Available when there is no Ready, is "True". It is false when there are no such conditions, Stalled or Reconciling is "True", Progressing or Synced is "False", or status.observedGeneration (or Ready's own observedGeneration) is behind metadata.generation. For a Deployment or StatefulSet use rolloutComplete as well: Available stays "True" while a rollout is stuck on old pods.

```text
{{ conditionsHealthy .helmRelease }}  → true
{{ conditionsHealthy .deployment }}  → true
```

### conditionsState

`conditionsState` reads an object's standard conditions into one of "healthy", "progressing", "failed" or "unknown", for any kind that reports them: Flux, cert-manager, Crossplane, Argo, Deployments and most operators. It tells "not yet" from "failed", which conditionsHealthy does not. The first rule that matches decides:

- progressing: status.observedGeneration (or Ready's own) is behind metadata.generation, so the status is about an older spec;
- failed: Stalled is "True", Synced is "False" or Progressing is "False";
- progressing: Reconciling is "True";
- healthy: Ready is "True";
- progressing: Ready is "False" while Progressing, Synced or Issuing is "True" (Crossplane creating, cert-manager issuing), or Ready is "Unknown";
- failed: Ready is "False" otherwise;
- without Ready: healthy when Available is "True", progressing when it is present but not "True";
- unknown: none of these conditions.

conditionsHealthy is true exactly when this is "healthy".

```text
{{ conditionsState .permission }}  → "failed"
{{ conditionsState .helmRelease }}  → "healthy"
{{ eq (conditionsState .permission) "failed" }}  → true
```

### conditionsStateMessage

`conditionsStateMessage` returns the message of the condition that decided conditionsState: Stalled, Synced, Progressing, Reconciling, Ready or Available. Returns "" when no condition decided (a status about an older spec, or no standard conditions) or the deciding condition has no message.

```text
{{ conditionsStateMessage .permission }}  → "failed create Permission, missing User"
```

### expectState

`expectState` judges one status value the way conditionsState judges conditions, for kinds that report a field instead of standard conditions: "healthy" when the value matches healthy, "failed" when it matches failed, and "progressing" otherwise, including when the value is absent or empty (not reported yet). healthy and failed are each one value or a list (list, toList or a Go slice); an empty failed means nothing counts as failed. Values are compared as text, so true matches "true" and 1 matches 1.0. healthy wins when a value is in both. Find the value with get, phase or status.

```text
{{ expectState (get .database "status" "applied") true false }}  → "healthy"
{{ expectState (phase .pod) "Running" (list "Failed" "Unknown") }}  → "healthy"
```

### firstState

`firstState` returns the first of its states that says something: the first that is neither "unknown" nor empty, or "unknown" when none does. It chains a general rule and a fallback, such as conditions first and a phase when the object has no conditions KEL knows.

```text
{{ firstState (conditionsState .database) (expectState (get .database "status" "applied") true false) }}  → "healthy"
```

### generation

`generation` returns `metadata.generation` as int64. Increments on every spec change. Returns `0` when absent.

```text
{{ generation .deployment }}  → 3
```

### get

`get` navigates a nested path through a Kubernetes object using variadic string segments. Returns `nil` if any segment is missing. Nil-safe at every level.

```text
{{ get .cronjob "status" "lastScheduleTime" }}  → "2026-05-19T10:00:00Z"
```

### getAnnotation

`getAnnotation` returns the value of a single annotation key from `metadata.annotations`. Returns `""` when the key is absent or the object has no annotations.

```text
{{ getAnnotation .deployment "example.com/owner" }}  → "team-payments"
```

### getAnnotationInt

`getAnnotationInt` returns an annotation value parsed as `int64`. Returns `0` when the key is absent or the value is non-numeric. Use with autoscale targets and thresholds stored in annotations.

```text
{{ getAnnotationInt .deployment "deployment.kubernetes.io/revision" }}  → 2
```

### getLabel

`getLabel` returns the value of a single label key from `metadata.labels`. Returns `""` when the key is absent or the object has no labels.

```text
{{ getLabel .deployment "app" }}                     → "web"
{{ getLabel .deployment "profile" | default "medium" }}  → "medium"
```

### getLabelInt

`getLabelInt` returns a label value parsed as `int64`. Returns `0` when the key is absent or the value is non-numeric.

```text
{{ getLabelInt .deployment "example.com/shards" }}  → 4
```

### getStatus

`getStatus` returns the string value of a specific key from `status`. Numeric and boolean values are converted to their string form. Returns `""` when the field is absent, nil, or a structured value (map, slice) — use `get` or `status` for those.

```text
{{ getStatus .pod "phase" }}  → "Running"
```

### hasAnnotation

`hasAnnotation` returns `true` when the object has the given annotation key with a non-empty value.

```text
{{ hasAnnotation .deployment "example.com/owner" }}  → true
```

### hasCondition

`hasCondition` returns `true` if `status.conditions` contains a condition of the given type with `status: "True"`.

```text
{{ hasCondition .deployment "Available" }}  → true
```

### hasLabel

`hasLabel` returns `true` when the object has the given label key with a non-empty value.

```text
{{ hasLabel .deployment "app" }}  → true
```

### hasStatus

`hasStatus` returns `true` when the given status field exists and is non-empty. Works for both scalar and structured (map, slice) fields.

```text
{{ hasStatus .service "loadBalancer" }}  → true
```

### isSynced

`isSynced` returns `true` when `metadata.generation == status.observedGeneration`, meaning the controller has fully processed the current spec. Returns `true` for resources without generation tracking (both 0).

```text
{{ isSynced .deployment }}  → true
```

### isTerminating

`isTerminating` returns `true` when `metadata.deletionTimestamp` is set — the object exists but is being deleted.

```text
{{ isTerminating .deployment }}  → false
```

### labelMatches

`labelMatches` returns `true` when the object's labels contain every key/value pair given as arguments. Extra labels on the object are ignored. Useful for checking if a resource would be selected by a label selector.

```text
{{ labelMatches .deployment "app" "web" "tier" "frontend" }}  → true
```

### labels

`labels` returns the `metadata.labels` map. Returns an empty map if absent.

```text
{{ mapGet (labels .deployment) "tier" }}  → "frontend"
```

### meta

`meta` returns the `metadata` map of a Kubernetes object. Returns an empty map if metadata is absent — never nil.

```text
{{ (meta .deployment).name }}  → "web"
```

### name

`name` returns metadata.name or an empty string.

```text
{{ name .deployment }}  → "web"
```

### namespace

`namespace` returns metadata.namespace or an empty string.

```text
{{ namespace .deployment }}  → "payments"
```

### observedGeneration

`observedGeneration` returns `status.observedGeneration` as int64. This is the generation the controller last acted on.

```text
{{ observedGeneration .deployment }}  → 3
```

### ownerKind

`ownerKind` returns the `kind` of the first `ownerReference`.

```text
{{ ownerKind .replicaset }}  → "Deployment"
```

### ownerName

`ownerName` returns the `name` of the first `ownerReference`.

```text
{{ ownerName .replicaset }}  → "web"
```

### phase

`phase` returns `status.phase` as a string. Returns `""` when absent.

```text
{{ phase .pod }}  → "Running"
```

### resourceCPU

`resourceCPU` returns `spec.resources.requests.cpu` from any object. Returns `""` when spec, resources, requests, or cpu is absent — no nil pointer panic at any level.

```text
{{ resourceCPU . }}  → "250m"
{{ resourceCPU .pod | default "100m" }}  → "100m"
```

### resourceExists

`resourceExists` reports whether obj is a Kubernetes object: a map with a metadata.name. A child that was never created, a nil, or an empty map is not.

```text
{{ resourceExists .deployment }}  → true
{{ resourceExists .missing }}     → false
```

### resourceMemory

`resourceMemory` returns `spec.resources.requests.memory` from any object. Returns `""` when any level is absent.

```text
{{ resourceMemory . }}  → "256Mi"
```

### spec

`spec` returns the `spec` map of a Kubernetes object. Returns an empty map if absent.

```text
{{ (spec .cronjob).schedule }}  → "0 * * * *"
```

### status

`status` returns the `status` map of a Kubernetes object. Returns an empty map if absent.

```text
{{ (status .deployment).readyReplicas }}  → 3
```
