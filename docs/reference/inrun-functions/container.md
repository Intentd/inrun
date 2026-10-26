# Container

Read the containers of a workload (a Deployment, StatefulSet, Job, Pod, …, through its pod template): images, environment variables, ports and counts, by container index or name.

## Reference

| Function | Description |
|----------|-------------|
| [`containerCount`](#containercount) | Returns the number of containers in the pod spec. |
| [`containerEnv`](#containerenv) | Returns the value of a named environment variable from the container at the given index. |
| [`containerImage`](#containerimage) | Returns the container image at the given index. |
| [`containerPort`](#containerport) | Returns a containerPort number: by default the first port of the first container; an optional container index and port index choose another. |
| [`containerPortByName`](#containerportbyname) | Returns the port number for a named container port. |
| [`containerPorts`](#containerports) | Returns the containerPort numbers the container at index exposes. |
| [`hasContainerPort`](#hascontainerport) | Reports whether the container exposes a specific port number. |

## Functions

### containerCount

`containerCount` returns the number of containers in the pod spec.

```text
{{ containerCount .deployment }}  → 2
```

### containerEnv

`containerEnv` returns the value of a named environment variable from the container at the given index.

```text
{{ containerEnv .deployment "APP_ENV" }}  → "production"
```

### containerImage

`containerImage` returns the container image at the given index.

```text
{{ containerImage .deployment }}    → "nginx:1.25"
{{ containerImage .deployment 1 }}  → "envoyproxy/envoy:v1.30"
```

### containerPort

`containerPort` returns a containerPort number: by default the first port of the first container; an optional container index and port index choose another. 0 when there is no such port.

```text
{{ containerPort .deployment }}  → 8080
```

### containerPortByName

`containerPortByName` returns the port number for a named container port. Returns 0 when not found.

```text
{{ containerPortByName .deployment 0 "metrics" }}  → 9090
```

### containerPorts

`containerPorts` returns the containerPort numbers the container at index exposes.

```text
{{ containerPorts .deployment 0 }}  → [8080 9090]
```

### hasContainerPort

`hasContainerPort` reports whether the container exposes a specific port number.

```text
{{ hasContainerPort .deployment 0 9090 }}  → true
```
