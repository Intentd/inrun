# Replica Notes

Replica notes read rollout state from Deployment, ReplicaSet, and StatefulSet objects. They replace verbose `get`/`mapGet` chains for the most commonly gated condition in async operator workflows: "is the rollout complete?"

---

## Reference

| Note | Description |
|------|-------------|
| `allReplicasReady` | Return `true` when `status. |
| `rolloutComplete` | Return `true` when `status. |
| `readyReplicas` | Return `status. |
| `availableReplicas` | Return `status. |
| `updatedReplicas` | Return `status. |
| `desiredReplicas` | Return `spec. |

## Examples

```yaml
# allReplicasReady
# Gate dependent resources on a stable rollout:
when:
  - field: "{{ allReplicasReady .children.deployment }}"
    equals: "true"

# rolloutComplete
# Check rollout progress without waiting for readiness:
- path: rolloutComplete
  value: "{{ rolloutComplete .children.deployment }}"

# readyReplicas
# value: "{{ readyReplicas .children.deployment }}"  → 3

# Surface in status:
- path: readyCount
  value: "{{ readyReplicas .children.deployment }}"

# availableReplicas
# value: "{{ availableReplicas .children.deployment }}"  → 3

# updatedReplicas
# value: "{{ updatedReplicas .children.deployment }}"  → 3

# Check rollout progress without waiting for readiness:
- path: rolloutComplete
  value: "{{ eq (updatedReplicas .children.deployment) (desiredReplicas .children.deployment) }}"

# desiredReplicas
# value: "{{ desiredReplicas .children.deployment }}"  → 3
status:
  fields:
    # Expose rollout progress as a fraction
    - path: rolloutProgress
      value: "{{ div (toFloat (readyReplicas .children.deployment)) (toFloat (desiredReplicas .children.deployment)) }}"

    # True only when all pods are both updated and ready
    - path: fullyRolledOut
      value: "{{ and (allReplicasReady .children.deployment) (rolloutComplete .children.deployment) }}"

when:
  - field: "{{ allReplicasReady .children.deployment }}"
    equals: "true"
```
