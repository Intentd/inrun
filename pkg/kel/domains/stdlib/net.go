package note

import (
	"net"
	"text/template"
)

func netNotes() template.FuncMap {
	return template.FuncMap{
		"cidrContains": noteCIDRContains,
		"ipValid":      noteIPValid,
		"ipIsPrivate":  noteIPIsPrivate,
	}
}

// noteCIDRContains reports whether ip falls within the CIDR block.
// Returns false for invalid input (safe zero value).
//
//	{{ cidrContains "10.0.0.0/8" "10.1.2.3" }}      → true
//	{{ cidrContains "192.168.0.0/16" "10.0.0.1" }}  → false
func noteCIDRContains(cidr, ip string) bool {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return network.Contains(parsed)
}

// noteIPValid reports whether s is a valid IPv4 or IPv6 address.
//
//	{{ ipValid "10.0.0.1" }}         → true
//	{{ ipValid "2001:db8::1" }}      → true
//	{{ ipValid "not-an-ip" }}        → false
func noteIPValid(s string) bool {
	return net.ParseIP(s) != nil
}

// noteIPIsPrivate reports whether the IP address is in a private (RFC 1918 / RFC 4193) range.
// Useful for network policy operators to differentiate internal from external addresses.
//
//	{{ ipIsPrivate "10.0.0.1" }}      → true
//	{{ ipIsPrivate "8.8.8.8" }}       → false
//	{{ ipIsPrivate "192.168.1.100" }} → true
func noteIPIsPrivate(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	for _, block := range private {
		_, network, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
