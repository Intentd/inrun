# Ingress Notes

Ingress notes surface routing rules, TLS configuration, and load-balancer assignment from Ingress objects.

---

## Reference

| Note | Description |
|------|-------------|
| `ingressReady` | Returns `true` when at least one load-balancer entry has been assigned — either an IP or hostname. |
| `ingressIP` | Returns `status. |
| `ingressHost` | Returns `status. |
| `ingressClassName` | Returns `spec. |
| `ingressRules` | Returns a comma-separated list of hostnames from `spec. |
| `ingressTLSHosts` | Returns a comma-separated list of TLS hostnames from `spec. |
| `ingressLoadBalancerIPs` | Returns a comma-joined list of all IPs and hostnames from `_loadBalancerIPs`. |
| `ingressTLSSecretCount` | Returns the number of TLS secrets fetched into `_tlsSecrets`. |

## Examples

```yaml
# ingressReady
when:
  - field: "{{ ingressReady .children.ingress }}"
    equals: "true"

# ingressIP
- path: externalIP
  value: "{{ ingressIP .children.ingress }}"
# → "34.123.45.67"

# ingressHost
- path: externalHost
  value: "{{ ingressHost .children.ingress }}"
# → "abc123.us-east-1.elb.amazonaws.com"

# ingressClassName
- path: ingressClass
  value: "{{ ingressClassName .children.ingress }}"
# → "nginx"

# ingressRules
- path: hosts
  value: "{{ ingressRules .children.ingress }}"
# → "api.example.com, www.example.com"

# ingressTLSHosts
- path: tlsHosts
  value: "{{ ingressTLSHosts .children.ingress }}"
# → "api.example.com, www.example.com"

# ingressLoadBalancerIPs
- path: lbAddresses
  value: "{{ ingressLoadBalancerIPs .children.ingress }}"
# → "1.2.3.4, abc.elb.amazonaws.com"

# ingressTLSSecretCount
- path: tlsSecretCount
  value: "{{ ingressTLSSecretCount .children.ingress }}"
# → 1
```
