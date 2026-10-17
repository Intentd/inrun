# Service Notes

Service notes surface networking details from Service and Endpoints objects — cluster IP, node ports, load balancer addresses, and endpoint readiness. Use them to gate Ingress creation, surface external access points in status fields, or verify that a Service is actually routing traffic before proceeding.

---

## Reference

| Note | Description |
|------|-------------|
| `serviceClusterIP` | Return `spec. |
| `serviceNodePort` | Return the `nodePort` of the first Service port. |
| `serviceLoadBalancerIP` | Return the external IP assigned by the load balancer (`status. |
| `serviceLoadBalancerHost` | Return the hostname assigned by the load balancer (`status. |
| `endpointsReady` | Return `true` when the Endpoints resource for a Service has at least one ready address. |
| `hasEndpoints` | Return `true` when at least one ready endpoint exists in `_endpoints`. |
| `serviceEndpoints` | Return all endpoints as a comma-separated `ip:port` list. |
| `serviceEndpointCount` | Return the total number of endpoints as `int`. |
| `serviceFirstEndpoint` | Return the first endpoint as `ip:port`. |
| `backingPodCount` | Return the number of pods selected by the Service's label selector. |
| `backingPodNames` | Return a comma-separated list of pod names selected by the Service. |

## Examples

```yaml
# serviceClusterIP
# value: "{{ serviceClusterIP .children.service }}"  → "10.96.0.1"

# Surface the cluster IP in status:
- path: clusterIP
  value: "{{ serviceClusterIP .children.service }}"

# serviceNodePort
# value: "{{ serviceNodePort .children.service }}"  → 31234

# serviceLoadBalancerIP
# value: "{{ serviceLoadBalancerIP .children.service }}"  → "34.123.45.67"

# Gate downstream on LB being ready:
when:
  - field: "{{ serviceLoadBalancerIP .children.service }}"
    notEquals: ""

# serviceLoadBalancerHost
# value: "{{ serviceLoadBalancerHost .children.service }}"
# → "abc123.us-east-1.elb.amazonaws.com"

# Compose a full URL:
- path: externalURL
  value: "https://{{ serviceLoadBalancerHost .children.service }}"

# endpointsReady
# Gate Ingress creation on actual backend availability:
when:
  - field: "{{ endpointsReady .children.service }}"
    equals: "true"

# hasEndpoints
when:
  - field: "{{ hasEndpoints .children.service }}"
    equals: "true"

# serviceEndpoints
- path: endpoints
  value: "{{ serviceEndpoints .children.service }}"
# → "10.0.0.1:8080, 10.0.0.2:8080"

# serviceEndpointCount
- path: endpointCount
  value: "{{ serviceEndpointCount .children.service }}"
# → 3

# serviceFirstEndpoint
- path: primaryEndpoint
  value: "{{ serviceFirstEndpoint .children.service }}"
# → "10.0.0.1:8080"

# backingPodCount
- path: backingPods
  value: "{{ backingPodCount .children.service }}"
# → 3

# backingPodNames
- path: backingPodNames
  value: "{{ backingPodNames .children.service }}"
# → "app-abc, app-def, app-ghi"
status:
  fields:
    - path: loadBalancerIP
      value: "{{ serviceLoadBalancerIP .children.service }}"
    - path: loadBalancerHost
      value: "{{ serviceLoadBalancerHost .children.service }}"

resources:
  - kind: Ingress
    name: app
    when:
      - field: "{{ serviceLoadBalancerHost .children.service }}"
        notEquals: ""
      - field: "{{ endpointsReady .children.service }}"
        equals: "true"
```
