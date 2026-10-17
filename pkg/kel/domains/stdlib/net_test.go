// pkg/note/net_test.go
package note

import (
	"testing"
)

func TestNoteCIDRContains(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		ip   string
		want bool
	}{
		{"valid IPv4 inside", "10.0.0.0/8", "10.1.2.3", true},
		{"valid IPv4 outside", "192.168.0.0/16", "10.0.0.1", false},
		{"valid IPv6 inside", "2001:db8::/32", "2001:db8::1", true},
		{"valid IPv6 outside", "2001:db8::/32", "2001:db9::1", false},
		{"invalid CIDR", "invalid", "10.0.0.1", false},
		{"invalid IP", "10.0.0.0/8", "invalid", false},
		{"empty cidr", "", "10.0.0.1", false},
		{"empty ip", "10.0.0.0/8", "", false},
		{"CIDR boundary", "10.0.0.0/24", "10.0.0.255", true},
		{"CIDR boundary excluded", "10.0.0.0/24", "10.0.1.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteCIDRContains(tt.cidr, tt.ip); got != tt.want {
				t.Errorf("noteCIDRContains(%q, %q) = %v, want %v", tt.cidr, tt.ip, got, tt.want)
			}
		})
	}
}

func TestNoteIPValid(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"valid IPv4", "10.0.0.1", true},
		{"valid IPv4 loopback", "127.0.0.1", true},
		{"valid IPv6", "2001:db8::1", true},
		{"valid IPv6 compressed", "::1", true},
		{"invalid", "not-an-ip", false},
		{"empty", "", false},
		{"IPv4 with leading zero", "0.0.0.0", true},
		{"IPv4 out of range", "256.0.0.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIPValid(tt.ip); got != tt.want {
				t.Errorf("noteIPValid(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestNoteIPIsPrivate(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"private IPv4 10.0.0.0/8", "10.0.0.1", true},
		{"private IPv4 172.16.0.0/12", "172.16.0.1", true},
		{"private IPv4 192.168.0.0/16", "192.168.1.100", true},
		{"public IPv4 8.8.8.8", "8.8.8.8", false},
		{"public IPv4 1.1.1.1", "1.1.1.1", false},
		{"private IPv6 fc00::/7", "fc00::1", true},
		{"private IPv6 fd12:3456:789a::1", "fd12:3456:789a::1", true},
		{"public IPv6 2001:db8::1", "2001:db8::1", false},
		{"invalid IP", "invalid", false},
		{"empty", "", false},
		{"127.0.0.1 (loopback, not private)", "127.0.0.1", false},
		{"169.254.0.1 (link-local, not private)", "169.254.0.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIPIsPrivate(tt.ip); got != tt.want {
				t.Errorf("noteIPIsPrivate(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
