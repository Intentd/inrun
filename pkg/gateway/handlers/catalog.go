// Package handlers serves the gateway's /catalog and /catalog/{crd}: the
// per-CRD admission, conversion and protection stats the gateway owns. The
// console merges them with the runtime's /catalog by GVR.
package handlers

import (
	"net/http"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/utils"
	"github.com/intentd/inrun/pkg/version"
)

// GatewayStatsProvider is the subset of webhook.WebhookServer the handlers
// need, declared here to avoid importing pkg/gateway/webhook.
type GatewayStatsProvider interface {
	AdmissionStatsFor(gvrKey string) *health.AdmissionStats
	ConversionStatsFor(gvrKey string) *health.ConversionStats
	ProtectionStatsFor(gvrKey string) *health.DeletionProtectionStats
	NamespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats
	InfraProtectionStats() *health.DeletionProtectionStats
	HousekeeperStats() *health.WebhookStats
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-field stat response types (JSON shapes shared with the runtime /catalog)
// ─────────────────────────────────────────────────────────────────────────────

type AdmissionStatsResponse struct {
	WebhooksEnabled   bool    `json:"webhooksEnabled"`
	ValidationTotal   int64   `json:"validationTotal"`
	ValidationAllowed int64   `json:"validationAllowed"`
	ValidationDenied  int64   `json:"validationDenied"`
	ValidationWarned  int64   `json:"validationWarned"`
	ValAvgLatencyMs   float64 `json:"valAvgLatencyMs"`
	ValP95LatencyMs   float64 `json:"valP95LatencyMs"`
	ValMaxLatencyMs   float64 `json:"valMaxLatencyMs"`
	MutationTotal     int64   `json:"mutationTotal"`
	MutationApplied   int64   `json:"mutationApplied"`
	MutationSkipped   int64   `json:"mutationSkipped"`
	MutAvgLatencyMs   float64 `json:"mutAvgLatencyMs"`
	MutP95LatencyMs   float64 `json:"mutP95LatencyMs"`
	MutMaxLatencyMs   float64 `json:"mutMaxLatencyMs"`
}

type ConversionStatsResponse struct {
	Enabled      bool  `json:"enabled"`
	Total        int64 `json:"total"`
	Success      int64 `json:"success"`
	Failures     int64 `json:"failures"`
	AvgLatencyMs int64 `json:"avgLatencyMs"`
	P95LatencyMs int64 `json:"p95LatencyMs"`
}

type DeletionProtectionStatsResponse struct {
	Enabled bool  `json:"enabled"`
	Total   int64 `json:"total"`
	Blocked int64 `json:"blocked"`
	Allowed int64 `json:"allowed"`
}

type NamespaceProtectionResponse struct {
	Enabled              bool     `json:"enabled"`
	HasNamespaceRules    bool     `json:"hasNamespaceRules"`
	Total                int64    `json:"total"`
	Blocked              int64    `json:"blocked"`
	Allowed              int64    `json:"allowed"`
	AllowedNamespaces    []string `json:"allowedNamespaces,omitempty"`
	RestrictedNamespaces []string `json:"restrictedNamespaces,omitempty"`
}

type HousekeeperStats struct {
	Reconciled int64 `json:"reconciled"`
	Failed     int64 `json:"failed"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────────────────────────────────────

// GatewayCatalogResponse is served at GET /catalog by the gateway process.
// It mirrors the top-level shape of CatalogResponse so console clients
// can pattern-match on the "source" field and merge stats by GVR key.
type GatewayCatalogResponse struct {
	Source  string `json:"source"` // always "gateway"
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`

	// Security feature flags — mirrors Catalog configuration.
	AdmissionEnabled           bool `json:"admissionEnabled"`
	ConversionEnabled          bool `json:"conversionEnabled"`
	DeletionProtectionEnabled  bool `json:"deletionProtectionEnabled"`
	NamespaceProtectionEnabled bool `json:"namespaceProtectionEnabled"`
	StrictModeEnabled          bool `json:"strictModeEnabled"`

	// Per-CRD stats — only gateway-owned fields populated.
	CRDs []GatewayCRDStatsResponse `json:"crds"`

	// Process-level stats for events not attributable to a single CRD:
	// the webhook configuration itself and Inrun infra resources.
	InfraProtection *DeletionProtectionStatsResponse `json:"infraProtection,omitempty"`
	Housekeeper     *HousekeeperStats                `json:"housekeeper,omitempty"`
	GatewayVersion  string                           `json:"gatewayVersion"`
}

// GatewayCRDStatsResponse holds the gateway-owned stats for one CRD.
// The GVR field is the merge key used by the console.
type GatewayCRDStatsResponse struct {
	Name                string                           `json:"name"`
	GVK                 string                           `json:"gvk"`
	GVR                 string                           `json:"gvr"` // merge key: "group/version/resource"
	Admission           *AdmissionStatsResponse          `json:"admission,omitempty"`
	Conversion          *ConversionStatsResponse         `json:"conversion,omitempty"`
	DeletionProtection  *DeletionProtectionStatsResponse `json:"deletionProtection,omitempty"`
	NamespaceProtection *NamespaceProtectionResponse     `json:"namespaceProtection,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// BuildGatewayCatalogHandler
// ─────────────────────────────────────────────────────────────────────────────

// BuildGatewayCatalogHandler returns the handler for GET /catalog on the gateway.
// It iterates the enabled CRDs from the Catalog, looks up per-CRD stats from ws,
// and returns a GatewayCatalogResponse.
func BuildGatewayCatalogHandler(cat *catalog.Catalog, ws GatewayStatsProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crds := buildGatewayCRDStats(cat, ws)

		resp := GatewayCatalogResponse{
			Source:                     "gateway",
			Name:                       cat.Meta().Name,
			Version:                    cat.Meta().Version,
			AdmissionEnabled:           cat.IsAdmissionEnabled(),
			ConversionEnabled:          cat.IsConversionEnabled(),
			DeletionProtectionEnabled:  cat.IsDeletionProtectionEnabled(),
			NamespaceProtectionEnabled: cat.IsNamespaceProtectionEnabled(),
			StrictModeEnabled:          cat.IsStrictModeEnabled(),
			CRDs:                       crds,
			GatewayVersion:             version.Short(),
		}

		if infra := ws.InfraProtectionStats(); infra != nil {
			snap := infra.GetStats()
			resp.InfraProtection = &DeletionProtectionStatsResponse{
				Enabled: cat.IsDeletionProtectionEnabled(),
				Total:   snap.TotalRequests,
				Blocked: snap.Blocked,
				Allowed: snap.Allowed,
			}
		}

		if wcs := ws.HousekeeperStats(); wcs != nil {
			snap := wcs.GetStats()
			resp.Housekeeper = &HousekeeperStats{
				Reconciled: snap.Reconciled,
				Failed:     snap.Failed,
			}
		}

		utils.WriteJSON(w, http.StatusOK, resp)
	}
}

// BuildGatewayCRDHandler returns the handler for GET /catalog/{crd} on the gateway.
// name, gvk, gvrStr, and gvrKey are derived from the CRDEntry at registration time.
func BuildGatewayCRDHandler(name, gvk, gvrStr, gvrKey string, ws GatewayStatsProvider, cat *catalog.Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := buildSingleGatewayCRDStats(name, gvk, gvrStr, gvrKey, ws, cat)
		utils.WriteJSON(w, http.StatusOK, resp)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildGatewayCRDStats(cat *catalog.Catalog, ws GatewayStatsProvider) []GatewayCRDStatsResponse {
	enabled := cat.Enabled()
	out := make([]GatewayCRDStatsResponse, 0, len(enabled))
	for _, crd := range enabled {
		if crd.IsBuiltIn {
			continue
		}
		gvr := crd.GVR()
		gvrKey := gvrKey(gvr.Group, gvr.Version, gvr.Resource)
		out = append(out, buildSingleGatewayCRDStats(
			crd.Name,
			crd.GVKString(),
			gvr.String(),
			gvrKey,
			ws,
			cat,
		))
	}
	return out
}

func buildSingleGatewayCRDStats(name, gvk, gvrStr, gvrKey string, ws GatewayStatsProvider, cat *catalog.Catalog) GatewayCRDStatsResponse {
	resp := GatewayCRDStatsResponse{
		Name: name,
		GVK:  gvk,
		GVR:  gvrStr,
	}

	// Admission stats — only when admission webhooks are enabled for this CRD.
	if cat.IsAdmissionEnabled() {
		if adm := ws.AdmissionStatsFor(gvrKey); adm != nil {
			snap := adm.GetStats(true)
			resp.Admission = &AdmissionStatsResponse{
				WebhooksEnabled:   true,
				ValidationTotal:   snap.ValidationTotal,
				ValidationAllowed: snap.ValidationAllowed,
				ValidationDenied:  snap.ValidationDenied,
				ValidationWarned:  snap.ValidationWarned,
				ValAvgLatencyMs:   snap.ValAvgLatencyMs,
				ValP95LatencyMs:   snap.ValP95LatencyMs,
				ValMaxLatencyMs:   snap.ValMaxLatencyMs,
				MutationTotal:     snap.MutationTotal,
				MutationApplied:   snap.MutationApplied,
				MutationSkipped:   snap.MutationSkipped,
				MutAvgLatencyMs:   snap.MutAvgLatencyMs,
				MutP95LatencyMs:   snap.MutP95LatencyMs,
				MutMaxLatencyMs:   snap.MutMaxLatencyMs,
			}
		}
	}

	// Conversion stats.
	if cat.IsConversionEnabled() {
		if conv := ws.ConversionStatsFor(gvrKey); conv != nil {
			snap := conv.GetStats()
			resp.Conversion = &ConversionStatsResponse{
				Enabled:      true,
				Total:        snap.TotalRequests,
				Success:      snap.SuccessRequests,
				Failures:     snap.FailedRequests,
				AvgLatencyMs: snap.AvgLatency.Milliseconds(),
				P95LatencyMs: snap.P95Latency.Milliseconds(),
			}
		} else {
			resp.Conversion = &ConversionStatsResponse{Enabled: true}
		}
	}

	// Deletion protection stats.
	if cat.IsDeletionProtectionEnabled() {
		if prot := ws.ProtectionStatsFor(gvrKey); prot != nil {
			snap := prot.GetStats()
			resp.DeletionProtection = &DeletionProtectionStatsResponse{
				Enabled: true,
				Total:   snap.TotalRequests,
				Blocked: snap.Blocked,
				Allowed: snap.Allowed,
			}
		} else {
			resp.DeletionProtection = &DeletionProtectionStatsResponse{Enabled: true}
		}
	}

	// Namespace protection stats.
	if cat.IsNamespaceProtectionEnabled() {
		if ns := ws.NamespaceStatsFor(gvrKey); ns != nil {
			snap := ns.GetStats()
			resp.NamespaceProtection = &NamespaceProtectionResponse{
				Enabled: true,
				Total:   snap.TotalRequests,
				Blocked: snap.Blocked,
				Allowed: snap.Allowed,
			}
		}
	}

	return resp
}

// gvrKey formats group/version/resource into the canonical stats key.
// Matches crdGVRKey in pkg/webhook so both sides produce the same string.
func gvrKey(group, version, resource string) string {
	if group == "" {
		return version + "/" + resource
	}
	return group + "/" + version + "/" + resource
}
