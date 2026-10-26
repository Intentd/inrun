package protection

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/util/kube"
)

// Log prefixes for each webhook.
const (
	deletionProtection   = "deletion-protection"
	namespaceProtection  = "namespace-protection"
	strictModeProtection = "strict-mode-protection"
)

// Handler serves the protection webhooks and counts their decisions.
type Handler struct {
	// protectedCRDs holds the full names (plural.group) of the CRDs protected from deletion.
	protectedCRDs map[string]struct{}
	// crdGVRKeys maps a CRD's full name to its kube.GVRKey, for its stats.
	crdGVRKeys map[string]string
	// namespaceRules maps a CRD's full name to its namespace rules.
	namespaceRules map[string]*NamespaceRules

	// Per-CRD stats keyed by kube.GVRKey.
	deletionStats  *health.StatsMap[health.DeletionProtectionStats]
	namespaceStats *health.StatsMap[health.NamespaceProtectionStats]
	// infraStats counts what no CRD owns: the webhook configurations and
	// Inrun's own Deployment, Service and so on.
	infraStats *health.DeletionProtectionStats
	// strictModeStats is not per CRD.
	strictModeStats *health.DeletionProtectionStats
}

// New builds the Handler for the Blueprint's protected CRDs and namespace rules.
func New(bp *blueprint.Blueprint) *Handler {
	h := &Handler{
		crdGVRKeys:      map[string]string{},
		deletionStats:   health.NewStatsMap(health.NewDeletionProtectionStats),
		namespaceStats:  health.NewStatsMap(health.NewNamespaceProtectionStats),
		infraStats:      health.NewDeletionProtectionStats(),
		strictModeStats: health.NewDeletionProtectionStats(),
	}

	// Enabled, not Spec.CRDs: only enabled entries have their GVR set.
	for _, crd := range bp.Enabled() {
		gvr := crd.GVR()
		key := kube.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		h.deletionStats.For(key)
		h.namespaceStats.For(key)
		h.crdGVRKeys[crd.APITypes.Plural+"."+crd.APITypes.Group] = key
	}

	if bp.IsDeletionProtectionEnabled() {
		h.protectedCRDs = bp.DeletionProtectedCRDNames()
	}

	if bp.IsNamespaceProtectionEnabled() {
		for name, entry := range bp.NamespaceProtectionRuleMap() {
			if h.namespaceRules == nil {
				h.namespaceRules = map[string]*NamespaceRules{}
			}
			h.namespaceRules[name] = newNamespaceRules(entry.Allowed, entry.Restricted)
		}
	}

	return h
}

// DeletionStatsFor returns the deletion-protection stats for gvrKey, or nil.
func (h *Handler) DeletionStatsFor(gvrKey string) *health.DeletionProtectionStats {
	return h.deletionStats.Get(gvrKey)
}

// NamespaceStatsFor returns the namespace-protection stats for gvrKey, or nil.
func (h *Handler) NamespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats {
	return h.namespaceStats.Get(gvrKey)
}

// InfraStats returns the deletion-protection stats for what no CRD owns.
func (h *Handler) InfraStats() *health.DeletionProtectionStats { return h.infraStats }

// StrictModeStats returns the strict-mode stats.
func (h *Handler) StrictModeStats() *health.DeletionProtectionStats { return h.strictModeStats }
