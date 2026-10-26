# Service

Read a Service (cluster IP, node port, load balancer IP and host) and its EndpointSlices, passed as one slice or a list: whether any endpoint is ready, and the endpoints as "ip:port".

## Reference

| Function | Description |
|----------|-------------|
| [`endpointsReady`](#endpointsready) | Reports whether an EndpointSlice (or a core/v1 Endpoints object) has at least one ready address. |
| [`hasEndpoints`](#hasendpoints) | Reports whether any endpoint in the EndpointSlices is ready. |
| [`serviceClusterIP`](#serviceclusterip) | Returns `spec.clusterIP`. |
| [`serviceEndpointCount`](#serviceendpointcount) | Returns the number of endpoints (address and port pairs), ready or not. |
| [`serviceEndpoints`](#serviceendpoints) | Returns the ready endpoints as "ip:port", one per address and port. |
| [`serviceFirstEndpoint`](#servicefirstendpoint) | Returns the first ready endpoint as "ip:port"; "" when none is ready. |
| [`serviceLoadBalancerHost`](#serviceloadbalancerhost) | Returns the hostname assigned by the load balancer (`status.loadBalancer.ingress[0].hostname`). |
| [`serviceLoadBalancerIP`](#serviceloadbalancerip) | Returns the external IP assigned by the load balancer (`status.loadBalancer.ingress[0].ip`). |
| [`serviceNodePort`](#servicenodeport) | Returns the `nodePort` of the first Service port. |

## Functions

### endpointsReady

`endpointsReady` reports whether an EndpointSlice (or a core/v1 Endpoints object) has at least one ready address. Pass the Service's EndpointSlice, not the Service.

```text
{{ endpointsReady (index .endpointSlices 0) }}  → true
```

### hasEndpoints

`hasEndpoints` reports whether any endpoint in the EndpointSlices is ready.

```text
{{ hasEndpoints .endpointSlices }}
```

### serviceClusterIP

`serviceClusterIP` returns `spec.clusterIP`. Returns `""` before the IP is assigned or when the Service is absent.

```text
{{ serviceClusterIP .service }}  → "10.96.14.2"
```

### serviceEndpointCount

`serviceEndpointCount` returns the number of endpoints (address and port pairs), ready or not.

```text
{{ serviceEndpointCount .endpointSlices }}  → 3
```

### serviceEndpoints

`serviceEndpoints` returns the ready endpoints as "ip:port", one per address and port.

```text
{{ serviceEndpoints .endpointSlices }}  → [10.0.0.1:8080 10.0.0.2:8080]
```

### serviceFirstEndpoint

`serviceFirstEndpoint` returns the first ready endpoint as "ip:port"; "" when none is ready.

```text
{{ serviceFirstEndpoint .endpointSlices }}  → "10.0.0.1:8080"
```

### serviceLoadBalancerHost

`serviceLoadBalancerHost` returns the hostname assigned by the load balancer (`status.loadBalancer.ingress[0].hostname`). Cloud providers (AWS, GCP) typically assign a hostname rather than an IP.

```text
{{ serviceLoadBalancerHost .service }}  → "abc123.us-east-1.elb.amazonaws.com"
https://{{ serviceLoadBalancerHost .service }}
```

### serviceLoadBalancerIP

`serviceLoadBalancerIP` returns the external IP assigned by the load balancer (`status.loadBalancer.ingress[0].ip`). Returns `""` while provisioning — cloud LB provisioning typically takes 30–90 seconds.

```text
{{ serviceLoadBalancerIP .service }}  → "34.123.45.67"
```

### serviceNodePort

`serviceNodePort` returns the `nodePort` of the first Service port. Returns `0` before the port is assigned.

```text
{{ serviceNodePort .service }}  → 31234
```
