package webhook

import "github.com/intentd/inrun/pkg/health"

// The stats the gateway's /blueprint reports, keyed by kube.GVRKey. Each
// returns nil for a CRD the Blueprint does not declare and no request named.

// AdmissionStatsFor returns the admission stats for gvrKey.
func (ws *Server) AdmissionStatsFor(gvrKey string) *health.AdmissionStats {
	return ws.admission.StatsFor(gvrKey)
}

// ConversionStatsFor returns the conversion stats for gvrKey.
func (ws *Server) ConversionStatsFor(gvrKey string) *health.ConversionStats {
	return ws.conversion.StatsFor(gvrKey)
}

// ProtectionStatsFor returns the deletion-protection stats for gvrKey.
func (ws *Server) ProtectionStatsFor(gvrKey string) *health.DeletionProtectionStats {
	return ws.protection.DeletionStatsFor(gvrKey)
}

// NamespaceStatsFor returns the namespace-protection stats for gvrKey.
func (ws *Server) NamespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats {
	return ws.protection.NamespaceStatsFor(gvrKey)
}

// InfraProtectionStats returns the deletion-protection stats for what no CRD
// owns: the webhook configurations and Inrun's own Deployment, Service and so on.
func (ws *Server) InfraProtectionStats() *health.DeletionProtectionStats {
	return ws.protection.InfraStats()
}

// StrictModeStats returns the strict-mode stats, which are not per CRD.
func (ws *Server) StrictModeStats() *health.DeletionProtectionStats {
	return ws.protection.StrictModeStats()
}
