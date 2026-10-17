# Prometheus Notes

Notes for reading the result of a `protocol: prometheus` external call. All notes accept the map injected under `.external.<name>` by the Prometheus client.

## Reference

| Note | Description |
|------|-------------|
| `promValue` | Returns the canonical scalar value from a Prometheus instant query result. |
| `promSum` | Sums all series values in a vector result. |
| `promMax` | Returns the maximum value across all series in a vector result. |
| `promAboveThreshold` | Returns `"true"` when the first-series value is strictly greater than the threshold, `"false"` otherwise. |
| `promBelowThreshold` | Returns `"true"` when the first-series value is strictly less than the threshold, `"false"` otherwise. |
| `promSeriesCount` | Returns the number of series in a vector result as a string. |
| `promLabelValues` | Returns a comma-separated string of the given label's values across all series. |

## Examples

```yaml
# promValue
# value: "{{ promValue .external.goroutines }}"

# promSum
# value: "{{ promSum .external.requestsTotal }}"

# promMax
# value: "{{ promMax .external.cpuUsage }}"

# promAboveThreshold
# value: "{{ promAboveThreshold .external.queueDepth 1000 }}"

# promBelowThreshold
# value: "{{ promBelowThreshold .external.errorRate 0.01 }}"

# promSeriesCount
# value: "{{ promSeriesCount .external.activePods }}"

# promLabelValues
# value: "{{ promLabelValues .external.pods \"namespace\" }}"
```
