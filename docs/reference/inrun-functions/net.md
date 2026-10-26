# Net

Check IP addresses and CIDR ranges: whether an address is valid, private, or inside a range.

## Reference

| Function | Description |
|----------|-------------|
| [`cidrContains`](#cidrcontains) | Reports whether an IP address falls within a CIDR block. |
| [`ipIsPrivate`](#ipisprivate) | Reports whether an IP address falls within a private (RFC 1918 / RFC 4193) range. |
| [`ipValid`](#ipvalid) | Reports whether a string is a valid IPv4 or IPv6 address. |

## Functions

### cidrContains

`cidrContains` reports whether an IP address falls within a CIDR block. Returns `false` for malformed input rather than an error.

```text
{{ cidrContains "10.0.0.0/8" "10.4.2.7" }}  → true
```

### ipIsPrivate

`ipIsPrivate` reports whether an IP address falls within a private (RFC 1918 / RFC 4193) range. Covers `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, and `fc00::/7`.

```text
{{ ipIsPrivate "192.168.1.20" }}  → true
```

### ipValid

`ipValid` reports whether a string is a valid IPv4 or IPv6 address.

```text
{{ ipValid "10.4.2.7" }}  → true
{{ ipValid "10.4.2" }}    → false
```
