# 14 — Service Notes

Service notes surface networking details from Service and Endpoints objects — cluster IP, node ports, load balancer addresses, and endpoint readiness. Use them to gate Ingress creation, surface external access points in status fields, or verify that a Service is actually routing traffic before proceeding.

---

## Reference

### `serviceClusterIP`

Return `spec.clusterIP`. Returns `""` before the IP is assigned or when the Service is absent.

Keywords: service, networking, cluster, ip, address, internal

```yaml
# value: "{{ serviceClusterIP .children.service }}"  → "10.96.0.1"

# Surface the cluster IP in status:
- path: clusterIP
  value: "{{ serviceClusterIP .children.service }}"
```

---

### `serviceNodePort`

Return the `nodePort` of the first Service port. Returns `0` before the port is assigned.

Keywords: service, networking, nodeport, port, int, external

```yaml
# value: "{{ serviceNodePort .children.service }}"  → 31234
```

---

### `serviceLoadBalancerIP`

Return the external IP assigned by the load balancer (`status.loadBalancer.ingress[0].ip`). Returns `""` while provisioning — cloud LB provisioning typically takes 30–90 seconds.

Keywords: service, networking, loadbalancer, ip, external, cloud, provisioning

```yaml
# value: "{{ serviceLoadBalancerIP .children.service }}"  → "34.123.45.67"

# Gate downstream on LB being ready:
when:
  - field: "{{ serviceLoadBalancerIP .children.service }}"
    notEquals: ""
```

---

### `serviceLoadBalancerHost`

Return the hostname assigned by the load balancer (`status.loadBalancer.ingress[0].hostname`). Cloud providers (AWS, GCP) typically assign a hostname rather than an IP.

Keywords: service, networking, loadbalancer, hostname, external, cloud, aws, gcp

```yaml
# value: "{{ serviceLoadBalancerHost .children.service }}"
# → "abc123.us-east-1.elb.amazonaws.com"

# Compose a full URL:
- path: externalURL
  value: "https://{{ serviceLoadBalancerHost .children.service }}"
```

---

### `endpointsReady`

Return `true` when the Endpoints resource for a Service has at least one ready address. Reads from the Endpoints object, not the Service itself.

Keywords: service, endpoints, ready, boolean, networking, traffic

```yaml
# Gate Ingress creation on actual backend availability:
when:
  - field: "{{ endpointsReady .children.service }}"
    equals: "true"
```

Note: access via a `cross:` declaration pointing to the Endpoints resource — the Endpoints object has the same name as the Service but is a separate resource.

---

## Enriched endpoint notes

Require `enrich: [endpoints]` on the CRD.

### `hasEndpoints`

Return `true` when at least one ready endpoint exists in `_endpoints`.

Keywords: service, endpoints, ready, boolean, enriched, traffic, networking

```yaml
when:
  - field: "{{ hasEndpoints .children.service }}"
    equals: "true"
```

---

### `serviceEndpoints`

Return all endpoints as a comma-separated `ip:port` list.

Keywords: service, endpoints, list, ip, port, enriched, networking, addresses

```yaml
- path: endpoints
  value: "{{ serviceEndpoints .children.service }}"
# → "10.0.0.1:8080, 10.0.0.2:8080"
```

---

### `serviceEndpointCount`

Return the total number of endpoints as `int`.

Keywords: service, endpoints, count, int, enriched, networking

```yaml
- path: endpointCount
  value: "{{ serviceEndpointCount .children.service }}"
# → 3
```

---

### `serviceFirstEndpoint`

Return the first endpoint as `ip:port`. Returns `""` when no endpoints exist.

Keywords: service, endpoints, first, ip, port, enriched, networking, primary

```yaml
- path: primaryEndpoint
  value: "{{ serviceFirstEndpoint .children.service }}"
# → "10.0.0.1:8080"
```

---

## Enriched backing-pod notes

Require `enrich: [backingpods]` on the CRD. The enrichment layer selects pods matching the Service's `spec.selector` and embeds them under `_backingPods`.

### `backingPodCount`

Return the number of pods selected by the Service's label selector.

Keywords: service, pods, selector, count, enriched, networking, backing

```yaml
- path: backingPods
  value: "{{ backingPodCount .children.service }}"
# → 3
```

---

### `backingPodNames`

Return a comma-separated list of pod names selected by the Service.

Keywords: service, pods, selector, names, enriched, networking, backing

```yaml
- path: backingPodNames
  value: "{{ backingPodNames .children.service }}"
# → "app-abc, app-def, app-ghi"
```

---

## Complete pattern: surface and gate on load balancer

```yaml
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

---

## Quick reference

| Note | Signature | Returns | Requires |
|------|-----------|---------|----------|
| `serviceClusterIP` | `(obj any)` | `string` | — |
| `serviceNodePort` | `(obj any)` | `int` | — |
| `serviceLoadBalancerIP` | `(obj any)` | `string` | — |
| `serviceLoadBalancerHost` | `(obj any)` | `string` | — |
| `endpointsReady` | `(obj any)` | `bool` | — |
| `hasEndpoints` | `(obj any)` | `bool` | `enrich: [endpoints]` |
| `serviceEndpoints` | `(obj any)` | `string` | `enrich: [endpoints]` |
| `serviceEndpointCount` | `(obj any)` | `int` | `enrich: [endpoints]` |
| `serviceFirstEndpoint` | `(obj any)` | `string` | `enrich: [endpoints]` |
| `backingPodCount` | `(obj any)` | `int` | `enrich: [backingpods]` |
| `backingPodNames` | `(obj any)` | `string` | `enrich: [backingpods]` |

---

**Next →** [15 — Field Notes](15-fields.md)
