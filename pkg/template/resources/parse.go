package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// parseBool interprets common boolean representations from template expressions.
func parseBool(s string) bool {
	switch s {
	case "true", "True", "TRUE", "1", "yes", "YES":
		return true
	default:
		return false
	}
}

// parsePort interprets common port representations from template expressions.
func parsePort(s string) int {
	var p int
	fmt.Sscanf(s, "%d", &p)
	return p
}

// parseReplicas parses a replica count string, defaulting to 1 when empty or unparseable.
// Zero is a valid replica count — used to scale StatefulSets and Deployments to zero.
func parseReplicas(s string) int32 {
	if s != "" {
		var r int
		if n, _ := fmt.Sscanf(s, "%d", &r); n == 1 && r >= 0 {
			return int32(r)
		}
	}
	return 1
}

// ParseProtocol converts a protocol string to a corev1.Protocol, for the
// Blueprint and intent paths alike.
// Accepted values: TCP (default), UDP, SCTP — case-insensitive.
// Empty string and anything unrecognised defaults to TCP, matching Kubernetes behaviour.
func ParseProtocol(s string) corev1.Protocol {
	switch strings.ToUpper(s) {
	case "UDP":
		return corev1.ProtocolUDP
	case "SCTP":
		return corev1.ProtocolSCTP
	default:
		return corev1.ProtocolTCP
	}
}
