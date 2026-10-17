# HPA Notes

HPA notes surface replica counts, scaling state, and scale-target details from HorizontalPodAutoscaler objects.

---

## Reference

| Note | Description |
|------|-------------|
| `hpaCurrentReplicas` | Returns `status. |
| `hpaDesiredReplicas` | Returns `status. |
| `hpaMinReplicas` | Returns `spec. |
| `hpaMaxReplicas` | Returns `spec. |
| `hpaScaling` | Returns `true` when the HPA has a valid metric (`ScalingActive=True`) **and** `currentReplicas != desiredReplicas` — a real scale-out or scale-in is in progress. |
| `hpaScalingActive` | Returns `true` when the `ScalingActive` condition is `True` — the HPA can read its metric and is permitted to scale. |
| `hpaAbleToScale` | Returns `true` when the `AbleToScale` condition is `True` — the scale target exists and is reachable. |
| `hpaScalingLimited` | Returns `true` when the `ScalingLimited` condition is `True` — the desired replica count was clamped by the min or max bound. |
| `hpaAtMax` | Returns `true` when `currentReplicas >= maxReplicas` — the HPA has hit its ceiling. |
| `hpaScaleTargetName` | Returns `_scaleTarget. |
| `hpaScaleTargetKind` | Returns `_scaleTarget. |
| `hpaMetricTypes` | Returns a comma-separated list of metric source types from `_currentMetrics`. |

## Examples

```yaml
# hpaCurrentReplicas
- path: currentReplicas
  value: "{{ hpaCurrentReplicas .children.hpa }}"
# → 3

# hpaDesiredReplicas
- path: desiredReplicas
  value: "{{ hpaDesiredReplicas .children.hpa }}"
# → 5

# hpaMinReplicas
- path: minReplicas
  value: "{{ hpaMinReplicas .children.hpa }}"
# → 2

# hpaMaxReplicas
- path: maxReplicas
  value: "{{ hpaMaxReplicas .children.hpa }}"
# → 10

# hpaScaling
when:
  - field: "{{ hpaScaling .children.hpa }}"
    equals: "false"

# hpaScalingActive
- path: metricsReady
  value: "{{ hpaScalingActive .children.hpa }}"

# hpaAbleToScale
- path: targetReachable
  value: "{{ hpaAbleToScale .children.hpa }}"

# hpaScalingLimited
- path: atBound
  value: "{{ hpaScalingLimited .children.hpa }}"

# hpaAtMax
- path: atCapacity
  value: "{{ hpaAtMax .children.hpa }}"

# hpaScaleTargetName
- path: scaleTargetName
  value: "{{ hpaScaleTargetName .children.hpa }}"
# → "my-app"

# hpaScaleTargetKind
- path: scaleTargetKind
  value: "{{ hpaScaleTargetKind .children.hpa }}"
# → "Deployment"

# hpaMetricTypes
- path: metricTypes
  value: "{{ hpaMetricTypes .children.hpa }}"
# → "Resource, External"
```
