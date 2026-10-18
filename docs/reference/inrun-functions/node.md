# Node

Read a Node: readiness, allocatable CPU and memory, conditions, taints, and where it is (zone, region, instance type, from the well-known topology labels); and which node a pod is scheduled on.

## Reference

| Function | Description |
|----------|-------------|
| [`nodeAllocatableCPU`](#nodeallocatablecpu) | Returns `status.allocatable.cpu`. |
| [`nodeAllocatableMemory`](#nodeallocatablememory) | Returns `status.allocatable.memory`. |
| [`nodeCondition`](#nodecondition) | Returns the `status` string of the named node condition, or `""` when absent — common types are `Ready`, `MemoryPressure`, and `DiskPressure`. |
| [`nodeInstanceType`](#nodeinstancetype) | Returns the node's instance type (label node.kubernetes.io/instance-type). |
| [`nodeReady`](#nodeready) | Returns `true` when the Node's Ready condition status is `"True"`. |
| [`nodeRegion`](#noderegion) | Returns the node's region (label topology.kubernetes.io/region). |
| [`nodeTaints`](#nodetaints) | Returns the keys of the node's taints. |
| [`nodeZone`](#nodezone) | Returns the node's zone (label topology.kubernetes.io/zone). |
| [`podNodeName`](#podnodename) | Returns the node a pod is scheduled on; "" while pending. |

## Functions

### nodeAllocatableCPU

`nodeAllocatableCPU` returns `status.allocatable.cpu`.

```text
{{ nodeAllocatableCPU .node }}  → "3920m"
```

### nodeAllocatableMemory

`nodeAllocatableMemory` returns `status.allocatable.memory`.

```text
{{ nodeAllocatableMemory .node }}  → "15032020Ki"
```

### nodeCondition

`nodeCondition` returns the `status` string of the named node condition, or `""` when absent — common types are `Ready`, `MemoryPressure`, and `DiskPressure`.

```text
{{ nodeCondition .node "MemoryPressure" }}  → "False"
```

### nodeInstanceType

`nodeInstanceType` returns the node's instance type (label node.kubernetes.io/instance-type).

```text
{{ nodeInstanceType .node }}  → "m5.large"
```

### nodeReady

`nodeReady` returns `true` when the Node's Ready condition status is `"True"`.

```text
{{ nodeReady .node }}  → true
```

### nodeRegion

`nodeRegion` returns the node's region (label topology.kubernetes.io/region).

```text
{{ nodeRegion .node }}  → "us-east-2"
```

### nodeTaints

`nodeTaints` returns the keys of the node's taints.

```text
{{ nodeTaints .node }}  → [dedicated]
```

### nodeZone

`nodeZone` returns the node's zone (label topology.kubernetes.io/zone).

```text
{{ nodeZone .node }}  → "us-east-2a"
```

### podNodeName

`podNodeName` returns the node a pod is scheduled on; "" while pending.

```text
{{ podNodeName .pod }}  → "ip-10-0-1-5.us-east-2.compute.internal"
```
