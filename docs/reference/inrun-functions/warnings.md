# Warnings

Answer questions about Kubernetes Events, as the API returns them (core/v1 or events.k8s.io/v1), passed as a list or an EventList. Only Warning events count, so a caller can pass all of an object's events.

## Reference

| Function | Description |
|----------|-------------|
| [`firstWarning`](#firstwarning) | Returns the message of the first Warning event; "" when there is none. |
| [`hasWarnings`](#haswarnings) | Reports whether any of the events is a Warning. |
| [`warningCount`](#warningcount) | Returns the number of Warning events. |
| [`warningMessages`](#warningmessages) | Returns the message of each Warning event, in order. |
| [`warningReasons`](#warningreasons) | Returns the reason of each Warning event, in order. |

## Functions

### firstWarning

`firstWarning` returns the message of the first Warning event; "" when there is none.

```text
{{ firstWarning .events }}  → "Failed to pull image registry.example.com/web:2.0: not found"
```

### hasWarnings

`hasWarnings` reports whether any of the events is a Warning.

```text
{{ hasWarnings .events }}
```

### warningCount

`warningCount` returns the number of Warning events.

```text
{{ warningCount .events }}  → 2
```

### warningMessages

`warningMessages` returns the message of each Warning event, in order.

```text
{{ warningMessages .events }}
→ [Back-off restarting failed container Liveness probe failed]
```

### warningReasons

`warningReasons` returns the reason of each Warning event, in order.

```text
{{ warningReasons .events }}  → [Failed BackOff]
```
