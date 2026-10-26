// Package stats serves the gateway's /blueprint and /blueprint/{crd}: the
// per-CRD admission, conversion and protection stats the gateway owns, and the
// housekeeper's sync counts. The console merges them with the runtime's
// /blueprint by GVR.
package stats

import (
	"net/http"

	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/version"

	"github.com/intentd/inrun/pkg/util/httpapi"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/health"
)

// WebhookStats is the part of webhook.Server the handlers read,
// declared here to avoid importing pkg/gateway/webhook.
type WebhookStats interface {
	AdmissionStatsFor(gvrKey string) *health.AdmissionStats
	ConversionStatsFor(gvrKey string) *health.ConversionStats
	ProtectionStatsFor(gvrKey string) *health.DeletionProtectionStats
	NamespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats
	InfraProtectionStats() *health.DeletionProtectionStats
}

// HousekeeperStats is the part of housekeeper.Housekeeper the handler reads.
type HousekeeperStats interface {
	Stats() *health.WebhookStats
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-field stat response types (JSON shapes shared with the runtime /blueprint)
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

type HousekeeperStatsResponse struct {
	Reconciled int64 `json:"reconciled"`
	Failed     int64 `json:"failed"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────────────────────────────────────

// GatewayBlueprintResponse is served at GET /blueprint by the gateway process.
// It mirrors the top-level shape of BlueprintResponse so console clients
// can pattern-match on the "source" field and merge stats by GVR key.
type GatewayBlueprintResponse struct {
	Source  string `json:"source"` // always "gateway"
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`

	// Security feature flags — mirrors Blueprint configuration.
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
	Housekeeper     *HousekeeperStatsResponse        `json:"housekeeper,omitempty"`
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
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

// Handler returns the handler for GET /blueprint on the gateway.
// It iterates the enabled CRDs from the Blueprint, looks up per-CRD stats from ws
// and the sync counts from hk, and returns a GatewayBlueprintResponse.
func Handler(bp *blueprint.Blueprint, ws WebhookStats, hk HousekeeperStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crds := buildGatewayCRDStats(bp, ws)

		resp := GatewayBlueprintResponse{
			Source:                     "gateway",
			Name:                       bp.Meta().Name,
			Version:                    bp.Meta().Version,
			AdmissionEnabled:           bp.IsAdmissionEnabled(),
			ConversionEnabled:          bp.IsConversionEnabled(),
			DeletionProtectionEnabled:  bp.IsDeletionProtectionEnabled(),
			NamespaceProtectionEnabled: bp.IsNamespaceProtectionEnabled(),
			StrictModeEnabled:          bp.IsStrictModeEnabled(),
			CRDs:                       crds,
			GatewayVersion:             version.Short(),
		}

		if infra := ws.InfraProtectionStats(); infra != nil {
			snap := infra.GetStats()
			resp.InfraProtection = &DeletionProtectionStatsResponse{
				Enabled: bp.IsDeletionProtectionEnabled(),
				Total:   snap.TotalRequests,
				Blocked: snap.Blocked,
				Allowed: snap.Allowed,
			}
		}

		if wcs := hk.Stats(); wcs != nil {
			snap := wcs.GetStats()
			resp.Housekeeper = &HousekeeperStatsResponse{
				Reconciled: snap.Reconciled,
				Failed:     snap.Failed,
			}
		}

		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// CRDHandler returns the handler for GET /blueprint/{crd} on the gateway.
// name, gvk, gvrStr, and gvrKey are derived from the CRDEntry at registration time.
func CRDHandler(name, gvk, gvrStr, gvrKey string, ws WebhookStats, bp *blueprint.Blueprint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := buildSingleGatewayCRDStats(name, gvk, gvrStr, gvrKey, ws, bp)
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildGatewayCRDStats(bp *blueprint.Blueprint, ws WebhookStats) []GatewayCRDStatsResponse {
	enabled := bp.Enabled()
	out := make([]GatewayCRDStatsResponse, 0, len(enabled))
	for _, crd := range enabled {
		if crd.IsBuiltIn {
			continue
		}
		gvr := crd.GVR()
		gvrKey := kube.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		out = append(out, buildSingleGatewayCRDStats(
			crd.Name,
			crd.GVKString(),
			gvr.String(),
			gvrKey,
			ws,
			bp,
		))
	}
	return out
}

func buildSingleGatewayCRDStats(name, gvk, gvrStr, gvrKey string, ws WebhookStats, bp *blueprint.Blueprint) GatewayCRDStatsResponse {
	resp := GatewayCRDStatsResponse{
		Name: name,
		GVK:  gvk,
		GVR:  gvrStr,
	}

	// Admission stats — only when admission webhooks are enabled for this CRD.
	if bp.IsAdmissionEnabled() {
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
	if bp.IsConversionEnabled() {
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
	if bp.IsDeletionProtectionEnabled() {
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
	if bp.IsNamespaceProtectionEnabled() {
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
