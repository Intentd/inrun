# 19 — HPA Notes

HPA notes surface replica counts, scaling state, and scale-target details from HorizontalPodAutoscaler objects.

---

## Reference

### `hpaCurrentReplicas`

Returns `status.currentReplicas` as `int64`.

Keywords: hpa, autoscaler, replicas, current, int, scale

```yaml
- path: currentReplicas
  value: "{{ hpaCurrentReplicas .children.hpa }}"
# → 3
```

---

### `hpaDesiredReplicas`

Returns `status.desiredReplicas` as `int64`.

Keywords: hpa, autoscaler, replicas, desired, int, scale, target

```yaml
- path: desiredReplicas
  value: "{{ hpaDesiredReplicas .children.hpa }}"
# → 5
```

---

### `hpaMinReplicas`

Returns `spec.minReplicas` as `int64`, defaulting to `1` when not set (matching the Kubernetes default).

Keywords: hpa, autoscaler, replicas, minimum, int, bound, floor

```yaml
- path: minReplicas
  value: "{{ hpaMinReplicas .children.hpa }}"
# → 2
```

---

### `hpaMaxReplicas`

Returns `spec.maxReplicas` as `int64`.

Keywords: hpa, autoscaler, replicas, maximum, int, bound, ceiling, capacity

```yaml
- path: maxReplicas
  value: "{{ hpaMaxReplicas .children.hpa }}"
# → 10
```

---

### `hpaScaling`

Returns `true` when the HPA has a valid metric (`ScalingActive=True`) **and** `currentReplicas != desiredReplicas` — a real scale-out or scale-in is in progress. Returns `false` when the metric source is unknown, which would otherwise cause a misleading `true` due to `desiredReplicas=0`.

Keywords: hpa, autoscaler, scaling, active, boolean, in-progress, replicas

```yaml
when:
  - field: "{{ hpaScaling .children.hpa }}"
    equals: "false"
```

---

### `hpaScalingActive`

Returns `true` when the `ScalingActive` condition is `True` — the HPA can read its metric and is permitted to scale. `False` when the metric source is unknown (`<unknown>/80%`) or the HPA is otherwise disabled.

Keywords: hpa, autoscaler, condition, metric, boolean, active, scalingactive

```yaml
- path: metricsReady
  value: "{{ hpaScalingActive .children.hpa }}"
```

---

### `hpaAbleToScale`

Returns `true` when the `AbleToScale` condition is `True` — the scale target exists and is reachable.

Keywords: hpa, autoscaler, condition, target, boolean, abletoscale, reachable

```yaml
- path: targetReachable
  value: "{{ hpaAbleToScale .children.hpa }}"
```

---

### `hpaScalingLimited`

Returns `true` when the `ScalingLimited` condition is `True` — the desired replica count was clamped by the min or max bound.

Keywords: hpa, autoscaler, condition, clamped, boolean, scalinglimited, bound

```yaml
- path: atBound
  value: "{{ hpaScalingLimited .children.hpa }}"
```

---

### `hpaAtMax`

Returns `true` when `currentReplicas >= maxReplicas` — the HPA has hit its ceiling.

Keywords: hpa, autoscaler, replicas, capacity, boolean, ceiling, max

```yaml
- path: atCapacity
  value: "{{ hpaAtMax .children.hpa }}"
```

---

### `hpaScaleTargetName`

Returns `_scaleTarget.name` — the name of the resource this HPA scales. Requires `enrich: [hpa]`.

Keywords: hpa, autoscaler, target, name, enriched, string, deployment

```yaml
- path: scaleTargetName
  value: "{{ hpaScaleTargetName .children.hpa }}"
# → "my-app"
```

---

### `hpaScaleTargetKind`

Returns `_scaleTarget.kind` — the kind of the resource this HPA scales. Requires `enrich: [hpa]`.

Keywords: hpa, autoscaler, target, kind, enriched, string, deployment

```yaml
- path: scaleTargetKind
  value: "{{ hpaScaleTargetKind .children.hpa }}"
# → "Deployment"
```

---

### `hpaMetricTypes`

Returns a comma-separated list of metric source types from `_currentMetrics`. Requires `enrich: [hpa]`.

Keywords: hpa, autoscaler, metrics, types, enriched, string, resource, external

```yaml
- path: metricTypes
  value: "{{ hpaMetricTypes .children.hpa }}"
# → "Resource, External"
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `hpaCurrentReplicas` | `(obj any)` | `int64` | none |
| `hpaDesiredReplicas` | `(obj any)` | `int64` | none |
| `hpaMinReplicas` | `(obj any)` | `int64` | none |
| `hpaMaxReplicas` | `(obj any)` | `int64` | none |
| `hpaScaling` | `(obj any)` | `bool` | none |
| `hpaScalingActive` | `(obj any)` | `bool` | none |
| `hpaAbleToScale` | `(obj any)` | `bool` | none |
| `hpaScalingLimited` | `(obj any)` | `bool` | none |
| `hpaAtMax` | `(obj any)` | `bool` | none |
| `hpaScaleTargetName` | `(obj any)` | `string` | `enrich: [hpa]` |
| `hpaScaleTargetKind` | `(obj any)` | `string` | `enrich: [hpa]` |
| `hpaMetricTypes` | `(obj any)` | `string` | `enrich: [hpa]` |

---

**Next →** [20 — Ingress Notes](20-ingress.md)
