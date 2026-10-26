package protection

import (
	"fmt"
	"net/http"

	"github.com/intentd/inrun/pkg/gateway/webhook/admission"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/util/kube"
)

// Namespace serves /namespace-protection: it rejects CREATE and UPDATE of a
// CR in a namespace its CRD's allowedNamespaces or restrictedNamespaces
// forbid.
func (h *Handler) Namespace(w http.ResponseWriter, r *http.Request) {
	rv := admission.Decode(w, r, namespaceProtection)
	if rv == nil {
		return
	}

	req := rv.Request

	if req.Operation != "CREATE" && req.Operation != "UPDATE" {
		admission.Allow(w, rv)
		return
	}

	rules, ok := h.namespaceRulesForCRD(req.Resource.Group, req.Resource.Resource)
	if !ok {
		admission.Allow(w, rv)
		return
	}

	ns := req.Namespace

	if !rules.IsNamespaceAllowed(ns) {
		logger.Info().
			Str("crd", req.Resource.Resource+"."+req.Resource.Group).
			Str("namespace", ns).
			Str("uid", req.UID).
			Msgf("%s: blocking CR creation/update in forbidden namespace", namespaceProtection)

		metrics.RecordNamespaceProtectionBlocked(req.Resource.Resource)
		h.namespaceStats.For(kube.GVRKey(req.Resource.Group, req.Resource.Version, req.Resource.Resource)).RecordBlocked()

		admission.Deny(w, rv, fmt.Sprintf(
			"\n\n[Inrun Security] Namespace %q is not permitted for this CRD.\n\n"+
				"To allow this namespace, update the CRD's allowedNamespaces or restrictedNamespaces.\n\n",
			ns,
		))
		return
	}

	h.namespaceStats.For(kube.GVRKey(req.Resource.Group, req.Resource.Version, req.Resource.Resource)).RecordAllowed()

	admission.Allow(w, rv)
}

func (h *Handler) namespaceRulesForCRD(group, plural string) (*NamespaceRules, bool) {
	if h.namespaceRules == nil {
		return nil, false
	}
	key := plural + "." + group
	r, ok := h.namespaceRules[key]
	return r, ok
}

// NamespaceRules holds the allow/restrict namespace sets for one CRD.
type NamespaceRules struct {
	Allowed    map[string]struct{}
	Restricted map[string]struct{}
}

// IsNamespaceAllowed returns true when the namespace passes the CRD's namespace rules.
func (r *NamespaceRules) IsNamespaceAllowed(ns string) bool {
	if len(r.Allowed) > 0 {
		_, ok := r.Allowed[ns]
		return ok
	}
	if len(r.Restricted) > 0 {
		_, forbidden := r.Restricted[ns]
		return !forbidden
	}
	return true
}

// newNamespaceRules builds the rules from the Blueprint's lists.
func newNamespaceRules(allowed, restricted []string) *NamespaceRules {
	r := &NamespaceRules{
		Allowed:    make(map[string]struct{}, len(allowed)),
		Restricted: make(map[string]struct{}, len(restricted)),
	}
	for _, ns := range allowed {
		r.Allowed[ns] = struct{}{}
	}
	for _, ns := range restricted {
		r.Restricted[ns] = struct{}{}
	}
	return r
}
