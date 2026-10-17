# 22 — Node Notes

Node notes read Node resource state directly (for operators that manage Nodes as children) and navigate `_node` topology summaries embedded in Pod children by `enrich: [node]`.

---

## Reference

### `nodeReady`

Returns `true` when the Node's Ready condition status is `"True"`.

Keywords: node, kubernetes, ready, condition, boolean, health, cluster

```yaml
when:
  - field: "{{ nodeReady .children.node }}"
    equals: "true"
```

---

### `nodeAllocatableCPU`

Returns `status.allocatable.cpu`.

Keywords: node, kubernetes, cpu, allocatable, resource, capacity, string

```yaml
- path: allocatableCPU
  value: "{{ nodeAllocatableCPU .children.node }}"
# → "3920m"
```

---

### `nodeAllocatableMemory`

Returns `status.allocatable.memory`.

Keywords: node, kubernetes, memory, allocatable, resource, capacity, string

```yaml
- path: allocatableMemory
  value: "{{ nodeAllocatableMemory .children.node }}"
# → "15032020Ki"
```

---

### `nodeCondition`

Returns the `status` string of the named node condition, or `""` when absent — common types are `Ready`, `MemoryPressure`, and `DiskPressure`.

Keywords: node, kubernetes, condition, status, string, pressure, health

```yaml
- path: memoryPressure
  value: "{{ nodeCondition .children.node \"MemoryPressure\" }}"
# → "False"
```

---

### `nodeTaints`

Returns a comma-separated list of taint keys on the node, or `""` when no taints are present.

Keywords: node, kubernetes, taints, scheduling, string, toleration, placement

```yaml
- path: taints
  value: "{{ nodeTaints .children.node }}"
# → "node.kubernetes.io/not-ready"
```

---

### `podNodeName`

Returns `_node.name` — the name of the node the Pod is scheduled on. Requires `enrich: [node]` on Pod children.

Keywords: node, pod, topology, name, enriched, string, scheduled, placement

```yaml
- path: nodeName
  value: "{{ podNodeName .children.pod }}"
# → "ip-10-0-1-5.us-east-2.compute.internal"
```

---

### `podNodeZone`

Returns `_node.zone` derived from the `topology.kubernetes.io/zone` label. Requires `enrich: [node]`.

Keywords: node, pod, topology, zone, enriched, string, availability, region

```yaml
- path: zone
  value: "{{ podNodeZone .children.pod }}"
# → "us-east-2a"
```

---

### `podNodeRegion`

Returns `_node.region` derived from the `topology.kubernetes.io/region` label. Requires `enrich: [node]`.

Keywords: node, pod, topology, region, enriched, string, cloud, geo

```yaml
- path: region
  value: "{{ podNodeRegion .children.pod }}"
# → "us-east-2"
```

---

### `podNodeInstanceType`

Returns `_node.instanceType` derived from the `node.kubernetes.io/instance-type` label. Requires `enrich: [node]`.

Keywords: node, pod, topology, instance, enriched, string, cloud, type, ec2

```yaml
- path: instanceType
  value: "{{ podNodeInstanceType .children.pod }}"
# → "t3.medium"
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `nodeReady` | `(obj any)` | `bool` | none |
| `nodeAllocatableCPU` | `(obj any)` | `string` | none |
| `nodeAllocatableMemory` | `(obj any)` | `string` | none |
| `nodeCondition` | `(obj any, type string)` | `string` | none |
| `nodeTaints` | `(obj any)` | `string` | none |
| `podNodeName` | `(obj any)` | `string` | `enrich: [node]` |
| `podNodeZone` | `(obj any)` | `string` | `enrich: [node]` |
| `podNodeRegion` | `(obj any)` | `string` | `enrich: [node]` |
| `podNodeInstanceType` | `(obj any)` | `string` | `enrich: [node]` |

---

**Next →** [23 — StatefulSet Notes](23-statefulset.md)
