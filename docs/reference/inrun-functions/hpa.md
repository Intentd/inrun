# HPA

Read a HorizontalPodAutoscaler: current, desired, minimum and maximum replicas, whether it is scaling or at its limit, its conditions, its scale target and the types of metric it reports.

## Reference

| Function | Description |
|----------|-------------|
| [`hpaAbleToScale`](#hpaabletoscale) | Returns `true` when the `AbleToScale` condition is `True` — the scale target exists and is reachable. |
| [`hpaAtMax`](#hpaatmax) | Returns `true` when `currentReplicas >= maxReplicas` — the HPA has hit its ceiling. |
| [`hpaCurrentReplicas`](#hpacurrentreplicas) | Returns `status.currentReplicas` as `int64`. |
| [`hpaDesiredReplicas`](#hpadesiredreplicas) | Returns `status.desiredReplicas` as `int64`. |
| [`hpaMaxReplicas`](#hpamaxreplicas) | Returns `spec.maxReplicas` as `int64`. |
| [`hpaMetricTypes`](#hpametrictypes) | Returns the type of each metric the HPA currently reports (status.currentMetrics): Resource, Pods, Object, External, ContainerResource. |
| [`hpaMinReplicas`](#hpaminreplicas) | Returns `spec.minReplicas` as `int64`, defaulting to `1` when not set (matching the Kubernetes default). |
| [`hpaScaleTargetKind`](#hpascaletargetkind) | Returns the kind of the workload the HPA scales (spec.scaleTargetRef.kind). |
| [`hpaScaleTargetName`](#hpascaletargetname) | Returns the name of the workload the HPA scales (spec.scaleTargetRef.name). |
| [`hpaScaling`](#hpascaling) | Returns `true` when the HPA has a valid metric (`ScalingActive=True`) **and** `currentReplicas != desiredReplicas` — a real scale-out or scale-in is in progress. |
| [`hpaScalingActive`](#hpascalingactive) | Returns `true` when the `ScalingActive` condition is `True` — the HPA can read its metric and is permitted to scale. |
| [`hpaScalingLimited`](#hpascalinglimited) | Returns `true` when the `ScalingLimited` condition is `True` — the desired replica count was clamped by the min or max bound. |

## Functions

### hpaAbleToScale

`hpaAbleToScale` returns `true` when the `AbleToScale` condition is `True` — the scale target exists and is reachable.

```text
{{ hpaAbleToScale .hpa }}  → true
```

### hpaAtMax

`hpaAtMax` returns `true` when `currentReplicas >= maxReplicas` — the HPA has hit its ceiling.

```text
{{ hpaAtMax .hpa }}  → false
```

### hpaCurrentReplicas

`hpaCurrentReplicas` returns `status.currentReplicas` as `int64`.

```text
{{ hpaCurrentReplicas .hpa }}  → 3
```

### hpaDesiredReplicas

`hpaDesiredReplicas` returns `status.desiredReplicas` as `int64`.

```text
{{ hpaDesiredReplicas .hpa }}  → 5
```

### hpaMaxReplicas

`hpaMaxReplicas` returns `spec.maxReplicas` as `int64`.

```text
{{ hpaMaxReplicas .hpa }}  → 10
```

### hpaMetricTypes

`hpaMetricTypes` returns the type of each metric the HPA currently reports (status.currentMetrics): Resource, Pods, Object, External, ContainerResource.

```text
{{ hpaMetricTypes .hpa }}  → [Resource External]
```

### hpaMinReplicas

`hpaMinReplicas` returns `spec.minReplicas` as `int64`, defaulting to `1` when not set (matching the Kubernetes default).

```text
{{ hpaMinReplicas .hpa }}  → 2
```

### hpaScaleTargetKind

`hpaScaleTargetKind` returns the kind of the workload the HPA scales (spec.scaleTargetRef.kind).

```text
{{ hpaScaleTargetKind .hpa }}  → "Deployment"
```

### hpaScaleTargetName

`hpaScaleTargetName` returns the name of the workload the HPA scales (spec.scaleTargetRef.name).

```text
{{ hpaScaleTargetName .hpa }}  → "web"
```

### hpaScaling

`hpaScaling` returns `true` when the HPA has a valid metric (`ScalingActive=True`) **and** `currentReplicas != desiredReplicas` — a real scale-out or scale-in is in progress. Returns `false` when the metric source is unknown, which would otherwise cause a misleading `true` due to `desiredReplicas=0`.

```text
{{ hpaScaling .hpa }}  → true
```

### hpaScalingActive

`hpaScalingActive` returns `true` when the `ScalingActive` condition is `True` — the HPA can read its metric and is permitted to scale. `False` when the metric source is unknown (`<unknown>/80%`) or the HPA is otherwise disabled.

```text
{{ hpaScalingActive .hpa }}  → true
```

### hpaScalingLimited

`hpaScalingLimited` returns `true` when the `ScalingLimited` condition is `True` — the desired replica count was clamped by the min or max bound.

```text
{{ hpaScalingLimited .hpa }}  → false
```
