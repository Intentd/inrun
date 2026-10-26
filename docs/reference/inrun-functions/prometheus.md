# Prometheus

Read the response of a Prometheus HTTP API instant query (/api/v1/query), the object with "status" and "data": the value, sums and maxima across series, thresholds, series counts and label values. No data gives zero values; a sample that does not parse is an error.

## Reference

| Function | Description |
|----------|-------------|
| [`promAboveThreshold`](#promabovethreshold) | Reports whether the query's value is above threshold. |
| [`promBelowThreshold`](#prombelowthreshold) | Reports whether the query's value is below threshold. |
| [`promLabelValues`](#promlabelvalues) | Returns the value of label on each series of a vector result, in order; series without the label are skipped. |
| [`promMax`](#prommax) | Returns the largest series value in the result. |
| [`promSeriesCount`](#promseriescount) | Returns the number of series in the result: 1 for a scalar, 0 when there is no data. |
| [`promSum`](#promsum) | Returns the sum of every series value in the result. |
| [`promValue`](#promvalue) | Returns the query's value: a scalar result, or the first series of a vector result. |

## Functions

### promAboveThreshold

`promAboveThreshold` reports whether the query's value is above threshold. False when there is no data; an error when threshold is not a number.

```text
{{ promAboveThreshold .requests 1000 }}  → false
```

### promBelowThreshold

`promBelowThreshold` reports whether the query's value is below threshold. False when there is no data; an error when threshold is not a number.

```text
{{ promBelowThreshold .errorRate 0.05 }}  → true
```

### promLabelValues

`promLabelValues` returns the value of label on each series of a vector result, in order; series without the label are skipped.

```text
{{ promLabelValues .requests "pod" }}  → [web-0 web-1]
```

### promMax

`promMax` returns the largest series value in the result. 0 when there is no data.

```text
{{ promMax .requests }}  → 800
```

### promSeriesCount

`promSeriesCount` returns the number of series in the result: 1 for a scalar, 0 when there is no data.

```text
{{ promSeriesCount .requests }}  → 2
```

### promSum

`promSum` returns the sum of every series value in the result. 0 when there is no data.

```text
{{ promSum .requests }}  → 1250
```

### promValue

`promValue` returns the query's value: a scalar result, or the first series of a vector result. 0 when there is no data; an error when the sample is not a number.

```text
{{ promValue .errorRate }}  → 0.02
```
