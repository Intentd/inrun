# 20 — Ingress Notes

Ingress notes surface routing rules, TLS configuration, and load-balancer assignment from Ingress objects.

---

## Reference

### `ingressReady`

Returns `true` when at least one load-balancer entry has been assigned — either an IP or hostname.

Keywords: ingress, networking, loadbalancer, ready, boolean, provisioned, assigned

```yaml
when:
  - field: "{{ ingressReady .children.ingress }}"
    equals: "true"
```

---

### `ingressIP`

Returns `status.loadBalancer.ingress[0].ip`, or `""` while the LB is provisioning.

Keywords: ingress, networking, loadbalancer, ip, address, external, string

```yaml
- path: externalIP
  value: "{{ ingressIP .children.ingress }}"
# → "34.123.45.67"
```

---

### `ingressHost`

Returns `status.loadBalancer.ingress[0].hostname` — cloud providers such as AWS ALB and GCP assign a hostname rather than an IP.

Keywords: ingress, networking, loadbalancer, hostname, external, cloud, aws, gcp, string

```yaml
- path: externalHost
  value: "{{ ingressHost .children.ingress }}"
# → "abc123.us-east-1.elb.amazonaws.com"
```

---

### `ingressClassName`

Returns `spec.ingressClassName`, or `""` when not set.

Keywords: ingress, networking, class, string, controller, nginx, traefik

```yaml
- path: ingressClass
  value: "{{ ingressClassName .children.ingress }}"
# → "nginx"
```

---

### `ingressRules`

Returns a comma-separated list of hostnames from `spec.rules`; empty-host catch-all rules are omitted.

Keywords: ingress, networking, rules, hosts, routing, string, list

```yaml
- path: hosts
  value: "{{ ingressRules .children.ingress }}"
# → "api.example.com, www.example.com"
```

---

### `ingressTLSHosts`

Returns a comma-separated list of TLS hostnames from `spec.tls`.

Keywords: ingress, networking, tls, hosts, ssl, string, list, https

```yaml
- path: tlsHosts
  value: "{{ ingressTLSHosts .children.ingress }}"
# → "api.example.com, www.example.com"
```

---

### `ingressLoadBalancerIPs`

Returns a comma-joined list of all IPs and hostnames from `_loadBalancerIPs`. Requires `enrich: [ingress]`.

Keywords: ingress, networking, loadbalancer, addresses, enriched, string, list

```yaml
- path: lbAddresses
  value: "{{ ingressLoadBalancerIPs .children.ingress }}"
# → "1.2.3.4, abc.elb.amazonaws.com"
```

---

### `ingressTLSSecretCount`

Returns the number of TLS secrets fetched into `_tlsSecrets`. Requires `enrich: [ingress]`.

Keywords: ingress, networking, tls, secrets, count, enriched, int, ssl

```yaml
- path: tlsSecretCount
  value: "{{ ingressTLSSecretCount .children.ingress }}"
# → 1
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `ingressReady` | `(obj any)` | `bool` | none |
| `ingressIP` | `(obj any)` | `string` | none |
| `ingressHost` | `(obj any)` | `string` | none |
| `ingressClassName` | `(obj any)` | `string` | none |
| `ingressRules` | `(obj any)` | `string` | none |
| `ingressTLSHosts` | `(obj any)` | `string` | none |
| `ingressLoadBalancerIPs` | `(obj any)` | `string` | `enrich: [ingress]` |
| `ingressTLSSecretCount` | `(obj any)` | `int` | `enrich: [ingress]` |

---

**Next →** [21 — PVC Notes](21-pvc.md)
