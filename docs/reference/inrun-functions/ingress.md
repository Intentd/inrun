# Ingress

Read an Ingress: whether it has a load balancer address, its host and IP, its class, the hosts in its rules and TLS section, and every load balancer address.

## Reference

| Function | Description |
|----------|-------------|
| [`ingressClassName`](#ingressclassname) | Returns `spec.ingressClassName`, or `""` when not set. |
| [`ingressHost`](#ingresshost) | Returns `status.loadBalancer.ingress[0].hostname` — cloud providers such as AWS ALB and GCP assign a hostname rather than an IP. |
| [`ingressIP`](#ingressip) | Returns `status.loadBalancer.ingress[0].ip`, or `""` while the LB is provisioning. |
| [`ingressLoadBalancerIPs`](#ingressloadbalancerips) | Returns each load balancer address in status.loadBalancer.ingress: its IP, or its hostname when it has no IP. |
| [`ingressReady`](#ingressready) | Returns `true` when at least one load-balancer entry has been assigned — either an IP or hostname. |
| [`ingressRules`](#ingressrules) | Returns the hostnames in spec.rules, in order. |
| [`ingressTLSHosts`](#ingresstlshosts) | Returns the distinct TLS hostnames in spec.tls. |

## Functions

### ingressClassName

`ingressClassName` returns `spec.ingressClassName`, or `""` when not set.

```text
{{ ingressClassName .ingress }}  → "nginx"
```

### ingressHost

`ingressHost` returns `status.loadBalancer.ingress[0].hostname` — cloud providers such as AWS ALB and GCP assign a hostname rather than an IP.

```text
{{ ingressHost .ingress }}  → "abc123.us-east-1.elb.amazonaws.com"
```

### ingressIP

`ingressIP` returns `status.loadBalancer.ingress[0].ip`, or `""` while the LB is provisioning.

```text
{{ ingressIP .ingress }}  → "34.123.45.67"
```

### ingressLoadBalancerIPs

`ingressLoadBalancerIPs` returns each load balancer address in status.loadBalancer.ingress: its IP, or its hostname when it has no IP.

```text
{{ ingressLoadBalancerIPs .ingress }}  → [34.123.45.67]
```

### ingressReady

`ingressReady` returns `true` when at least one load-balancer entry has been assigned — either an IP or hostname.

```text
{{ ingressReady .ingress }}  → true
```

### ingressRules

`ingressRules` returns the hostnames in spec.rules, in order. Empty hosts (catch-all rules) are omitted.

```text
{{ ingressRules .ingress }}  → [api.example.com www.example.com]
```

### ingressTLSHosts

`ingressTLSHosts` returns the distinct TLS hostnames in spec.tls.

```text
{{ ingressTLSHosts .ingress }}  → [api.example.com]
```
