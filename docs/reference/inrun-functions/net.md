# Network Notes

Network notes work with IP addresses and CIDR blocks. They provide safe, zero-panic helpers for the common network-policy and routing decisions an operator needs to make — whether an IP belongs to a subnet, whether a string is a valid address, and whether an address is internal.

---

## Reference

| Note | Description |
|------|-------------|
| `cidrContains` | Report whether an IP address falls within a CIDR block. |
| `ipValid` | Report whether a string is a valid IPv4 or IPv6 address. |
| `ipIsPrivate` | Report whether an IP address falls within a private (RFC 1918 / RFC 4193) range. |

## Examples

```yaml
# cidrContains
# Allow ingress only from the internal pod CIDR
when:
  - field: "{{ cidrContains \"10.0.0.0/8\" .spec.clientIP }}"
    equals: "true"
{{ cidrContains "10.0.0.0/8"      "10.1.2.3"   }}  → true
{{ cidrContains "192.168.0.0/16"  "10.0.0.1"   }}  → false
{{ cidrContains "bad-cidr"        "10.0.0.1"   }}  → false

# ipValid
validation:
  rules:
    - field: "{{ ipValid .spec.clientIP }}"
      equals: "true"
      message: "spec.clientIP must be a valid IP address"
      action: deny
{{ ipValid "10.0.0.1"     }}  → true
{{ ipValid "2001:db8::1"  }}  → true
{{ ipValid "not-an-ip"    }}  → false
{{ ipValid ""             }}  → false

# ipIsPrivate
# Route traffic differently based on whether the source is internal
- path: trafficClass
  value: "{{ if ipIsPrivate .spec.sourceIP }}internal{{ else }}external{{ end }}"
{{ ipIsPrivate "10.0.0.1"       }}  → true
{{ ipIsPrivate "192.168.1.100"  }}  → true
{{ ipIsPrivate "172.20.0.5"     }}  → true
{{ ipIsPrivate "8.8.8.8"        }}  → false
{{ ipIsPrivate "2001:db8::1"    }}  → false
```
