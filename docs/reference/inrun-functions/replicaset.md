# ReplicaSet

Read a Deployment's ReplicaSets, passed as a list: how many, their names, and which are older than the Deployment's current revision; and any object's controller, from its ownerReferences.

## Reference

| Function | Description |
|----------|-------------|
| [`deploymentReplicaSetCount`](#deploymentreplicasetcount) | Returns the number of ReplicaSets, including scaled-to-zero ones a Deployment keeps for rollback. |
| [`deploymentReplicaSets`](#deploymentreplicasets) | Returns the ReplicaSets' names. |
| [`oldDeploymentReplicaSets`](#olddeploymentreplicasets) | Returns the names of the deployment's ReplicaSets that are not its current revision: those whose deployment.kubernetes.io/revision annotation differs from the Deployment's. |
| [`replicaSetOwnerKind`](#replicasetownerkind) | Returns the kind of the object's controller; for a ReplicaSet, typically "Deployment". |
| [`replicaSetOwnerName`](#replicasetownername) | Returns the name of the object's controller (the ownerReference with controller: true); for a ReplicaSet, its Deployment. |

## Functions

### deploymentReplicaSetCount

`deploymentReplicaSetCount` returns the number of ReplicaSets, including scaled-to-zero ones a Deployment keeps for rollback.

```text
{{ deploymentReplicaSetCount .replicaSets }}  → 2
```

### deploymentReplicaSets

`deploymentReplicaSets` returns the ReplicaSets' names.

```text
{{ deploymentReplicaSets .replicaSets }}  → [web-6d4f web-5c8b]
```

### oldDeploymentReplicaSets

`oldDeploymentReplicaSets` returns the names of the deployment's ReplicaSets that are not its current revision: those whose deployment.kubernetes.io/revision annotation differs from the Deployment's. Empty until the Deployment has a revision.

```text
{{ oldDeploymentReplicaSets .deployment .replicaSets }}  → [web-5c8b]
```

### replicaSetOwnerKind

`replicaSetOwnerKind` returns the kind of the object's controller; for a ReplicaSet, typically "Deployment".

```text
{{ replicaSetOwnerKind .replicaset }}  → "Deployment"
```

### replicaSetOwnerName

`replicaSetOwnerName` returns the name of the object's controller (the ownerReference with controller: true); for a ReplicaSet, its Deployment.

```text
{{ replicaSetOwnerName .replicaset }}  → "web"
```
