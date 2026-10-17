# Node Notes

Node notes read Node resource state directly (for operators that manage Nodes as children) and navigate `_node` topology summaries embedded in Pod children by `enrich: [node]`.

---

## Reference

| Note | Description |
|------|-------------|
| `nodeReady` | Returns `true` when the Node's Ready condition status is `"True"`. |
| `nodeAllocatableCPU` | Returns `status. |
| `nodeAllocatableMemory` | Returns `status. |
| `nodeCondition` | Returns the `status` string of the named node condition, or `""` when absent — common types are `Ready`, `MemoryPressure`, and `DiskPressure`. |
| `nodeTaints` | Returns a comma-separated list of taint keys on the node, or `""` when no taints are present. |
| `podNodeName` | Returns `_node. |
| `podNodeZone` | Returns `_node. |
| `podNodeRegion` | Returns `_node. |
| `podNodeInstanceType` | Returns `_node. |

## Examples

```yaml
# nodeReady
when:
  - field: "{{ nodeReady .children.node }}"
    equals: "true"

# nodeAllocatableCPU
- path: allocatableCPU
  value: "{{ nodeAllocatableCPU .children.node }}"
# → "3920m"

# nodeAllocatableMemory
- path: allocatableMemory
  value: "{{ nodeAllocatableMemory .children.node }}"
# → "15032020Ki"

# nodeCondition
- path: memoryPressure
  value: "{{ nodeCondition .children.node \"MemoryPressure\" }}"
# → "False"

# nodeTaints
- path: taints
  value: "{{ nodeTaints .children.node }}"
# → "node.kubernetes.io/not-ready"

# podNodeName
- path: nodeName
  value: "{{ podNodeName .children.pod }}"
# → "ip-10-0-1-5.us-east-2.compute.internal"

# podNodeZone
- path: zone
  value: "{{ podNodeZone .children.pod }}"
# → "us-east-2a"

# podNodeRegion
- path: region
  value: "{{ podNodeRegion .children.pod }}"
# → "us-east-2"

# podNodeInstanceType
- path: instanceType
  value: "{{ podNodeInstanceType .children.pod }}"
# → "t3.medium"
```
