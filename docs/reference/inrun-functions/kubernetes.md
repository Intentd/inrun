# Kubernetes Notes

Kubernetes notes safely navigate the unstructured objects that Inrun exposes through the template context — especially child resources available under `.children.*` and cross-CRD observations under `.cross.*`.

## Reference

| Note | Description |
|------|-------------|
| `meta` | Return the `metadata` map of a Kubernetes object. |
| `labels` | Return the `metadata. |
| `annotations` | Return the `metadata. |
| `getLabel` | Return the value of a single label key from `metadata. |
| `getLabelInt` | Return a label value parsed as `int64`. |
| `hasLabel` | Return `true` when the object has the given label key with a non-empty value. |
| `getAnnotation` | Return the value of a single annotation key from `metadata. |
| `getAnnotationInt` | Return an annotation value parsed as `int64`. |
| `hasAnnotation` | Return `true` when the object has the given annotation key with a non-empty value. |
| `labelMatches` | Return `true` when the object's labels contain every key/value pair given as arguments. |
| `spec` | Return the `spec` map of a Kubernetes object. |
| `status` | Return the `status` map of a Kubernetes object. |
| `getStatus` | Return the string value of a specific key from `status`. |
| `hasStatus` | Return `true` when the given status field exists and is non-empty. |
| `phase` | Return `status. |
| `get` | Navigate a nested path through a Kubernetes object using variadic string segments. |
| `ownerKind` | Return the `kind` of the first `ownerReference`. |
| `ownerName` | Return the `name` of the first `ownerReference`. |
| `hasCondition` | Return `true` if `status. |
| `conditionReason` | Return the `reason` field of a named condition. |
| `conditionMessage` | Return the `message` field of a named condition. |
| `resourceExists` | Return `true` when the object is a non-nil `map[string]interface{}`. |
| `isTerminating` | Return `true` when `metadata. |
| `generation` | Return `metadata. |
| `observedGeneration` | Return `status. |
| `isSynced` | Return `true` when `metadata. |
| `resourceCPU` | Return `spec. |
| `resourceMemory` | Return `spec. |

## Examples

```yaml
# meta
# value: "{{ meta .children.cronjob }}"
# Useful as an intermediate value when chaining with mapGet:
# value: "{{ mapGet (meta .children.cronjob) \"resourceVersion\" }}"

# labels
# value: "{{ mapGet (labels .children.deployment) \"app\" }}"
# {app: frontend, tier: web} → "frontend"

# annotations
# value: "{{ mapGet (annotations .children.deployment) \"inrun.dev/phase\" }}"

# getLabel
# value: "{{ getLabel .children.deployment \"app.kubernetes.io/name\" }}"
# → "my-app"

# Drive logic from a label without a spec field:
# value: '{{ getLabel . "inrun.dev/resource-profile" | default "medium" }}'

# getLabelInt
# value: "{{ getLabelInt .children.deployment \"replica-count\" }}"
# → 3

# hasLabel
# when:
#   - field: '{{ hasLabel .children.deployment "app.kubernetes.io/managed-by" }}'
#     equals: "true"

# getAnnotation
# value: '{{ getAnnotation . "autoscale/min-replicas" | default "2" }}'
# → "2"

# getAnnotationInt
# autoscale:
#   scaleDown:
#     target: '{{ getAnnotationInt . "autoscale/min-replicas" | default 2 }}'

# hasAnnotation
# when:
#   - field: '{{ hasAnnotation . "autoscale/enabled" }}'
#     equals: "true"

# labelMatches
# value: '{{ labelMatches .children.deployment "app" "frontend" "env" "prod" }}'
# → true when the deployment carries both labels

# spec
# value: "{{ mapGet (spec .children.cronjob) \"schedule\" }}"
# Equivalent to: .children.cronjob.spec.schedule

# status
# value: "{{ mapGet (status .children.deployment) \"readyReplicas\" }}"
# Equivalent to: .children.deployment.status.readyReplicas

# getStatus
# value: "{{ getStatus .children.deployment \"readyReplicas\" }}"
# → "3"

# when:
#   - field: '{{ getStatus .children.pod "phase" }}'
#     equals: "Running"

# hasStatus
# value: '{{ hasStatus .children.service "loadBalancer" }}'
# → true when loadBalancer is set in status

# when:
#   - field: '{{ hasStatus .children.deployment "readyReplicas" }}'
#     equals: "true"

# phase
# value: "{{ phase .children.pod }}"
# → "Running", "Pending", "Succeeded", "Failed", or ""

# when:
#   - field: "{{ phase .children.pod }}"
#     equals: "Running"

# get
# value: "{{ get .children.cronjob \"status\" \"lastScheduleTime\" }}"
# value: "{{ get .children.deployment \"spec\" \"template\" \"spec\" \"containers\" }}"

# ownerKind
# value: "{{ ownerKind .children.replicaset }}"
# → "Deployment"

# ownerName
# value: "{{ ownerName .children.replicaset }}"
# → "my-app"

# hasCondition
# value: "{{ hasCondition .children.deployment \"Available\" }}"
# Standard types: Available, Progressing, Degraded, Ready, Complete

# conditionReason
# value: "{{ conditionReason .children.deployment \"Available\" }}"
# → "MinimumReplicasAvailable" or ""

# conditionMessage
# value: "{{ conditionMessage .children.deployment \"Progressing\" }}"
# → "ReplicaSet has successfully progressed." or ""

# resourceExists
# value: "{{ resourceExists .children.deployment }}"

# Gate dependent resources on child existence:
# when:
#   - field: "{{ resourceExists .children.secret }}"
#     equals: "true"

# isTerminating
# Gate traffic routing away from terminating pods:
# when:
#   - field: "{{ isTerminating .children.deployment }}"
#     equals: "false"

# generation
# value: "{{ generation .children.deployment }}"
# → 3

# observedGeneration
# value: "{{ observedGeneration .children.deployment }}"
# → 3

# isSynced
# Gate dependent resources on rollout completion:
# when:
#   - field: "{{ isSynced .children.deployment }}"
#     equals: "true"
status:
  fields:
    - path: lastScheduleTime
      value: "{{ get .children.cronjob \"status\" \"lastScheduleTime\" }}"
    - path: deploymentReady
      value: "{{ hasCondition .children.deployment \"Available\" }}"
    - path: phase
      value: "{{ phase .children.pod }}"

when:
  - field: "{{ isSynced .children.deployment }}"
    equals: "true"
  - field: "{{ resourceExists .children.secret }}"
    equals: "true"

# resourceCPU
# normalize block — default resource requests without nil pointer panics
normalize:
  spec:
    resources.requests.cpu:    '{{ resourceCPU . | default "100m" }}'
    resources.requests.memory: '{{ resourceMemory . | default "128Mi" }}'

# resourceMemory
- path: memoryRequest
  value: "{{ resourceMemory .children.deployment }}"
```
