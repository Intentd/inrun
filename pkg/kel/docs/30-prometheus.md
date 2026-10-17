# 30 — Prometheus Notes

Notes for reading the result of a `protocol: prometheus` external call. All notes accept the map injected under `.external.<name>` by the Prometheus client.

## Reference

### `promValue`

Returns the canonical scalar value from a Prometheus instant query result. For scalar results it returns the value directly; for vector results it returns the value from the first series.

Keywords: prometheus, metric, scalar, vector, value, string, external

```yaml
# value: "{{ promValue .external.goroutines }}"
```

---

### `promSum`

Sums all series values in a vector result. Useful for aggregating per-pod or per-instance metrics across the cluster. Returns the value as a string (integer when no decimals, float otherwise).

Keywords: prometheus, vector, sum, aggregate, multi-series, string, external

```yaml
# value: "{{ promSum .external.requestsTotal }}"
```

---

### `promMax`

Returns the maximum value across all series in a vector result.

Keywords: prometheus, vector, max, maximum, peak, string, external

```yaml
# value: "{{ promMax .external.cpuUsage }}"
```

---

### `promAboveThreshold`

Returns `"true"` when the first-series value is strictly greater than the threshold, `"false"` otherwise. The threshold can be an integer, float64, or numeric string. Returns `"false"` on parse errors or empty results.

Keywords: prometheus, threshold, comparison, gate, boolean, string, external

```yaml
# value: "{{ promAboveThreshold .external.queueDepth 1000 }}"
```

---

### `promBelowThreshold`

Returns `"true"` when the first-series value is strictly less than the threshold, `"false"` otherwise. Mirrors `promAboveThreshold` for lower-bound gates — useful for alerting when a metric drops under a minimum.

Keywords: prometheus, threshold, comparison, gate, minimum, boolean, string, external

```yaml
# value: "{{ promBelowThreshold .external.errorRate 0.01 }}"
```

---

### `promSeriesCount`

Returns the number of series in a vector result as a string. Useful for asserting that a metric exists (`"1"`) or is absent (`"0"`).

Keywords: prometheus, series, count, vector, cardinality, string, external

```yaml
# value: "{{ promSeriesCount .external.activePods }}"
```

---

### `promLabelValues`

Returns a comma-separated string of the given label's values across all series. Useful for surfacing which namespaces, pods, or instances are represented in the result.

Keywords: prometheus, labels, label, vector, series, string, external

```yaml
# value: "{{ promLabelValues .external.pods \"namespace\" }}"
```

---

## Quick reference

| Note | Accepts | Returns | Use in |
|------|---------|---------|--------|
| `promValue` | `.external.<name>` map | string — first series value | status fields, when conditions |
| `promSum` | `.external.<name>` map | string — sum of all series | status fields |
| `promMax` | `.external.<name>` map | string — max of all series | status fields |
| `promAboveThreshold` | `.external.<name>` map, threshold | `"true"` / `"false"` | when conditions, status flags |
| `promBelowThreshold` | `.external.<name>` map, threshold | `"true"` / `"false"` | when conditions, status flags |
| `promSeriesCount` | `.external.<name>` map | string — series count | assertions, existence checks |
| `promLabelValues` | `.external.<name>` map, label | string — comma-separated values | status fields |
