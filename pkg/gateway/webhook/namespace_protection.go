// webhook/namespace_protection.go — /namespace-protection webhook handler.
//
// Registered only when security.namespaceProtection.enabled: true in the Blueprint.
// Intercepts CREATE and UPDATE on CRDs that declare allowedNamespaces or
// restrictedNamespaces, and rejects operations in forbidden namespaces.
//
// failurePolicy: Fail — if Inrun is unreachable, the operation is blocked.
package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

func (ws *WebhookServer) namespaceProtectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	var review AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logger.Error().Err(err).Msgf("%s: failed to decode AdmissionReview", namespaceProtection)
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	req := review.Request

	if req.Operation != "CREATE" && req.Operation != "UPDATE" {
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
		return
	}

	rules, ok := ws.namespaceRulesForCRD(req.Resource.Group, req.Resource.Resource)
	if !ok {
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
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
		ws.namespaceStatsFor(gvrToKey(req.Resource)).RecordBlocked()

		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Status: &AdmissionStatus{
				Message: fmt.Sprintf(
					"\n\n[Inrun Security] Namespace %q is not permitted for this CRD.\n\n"+
						"To allow this namespace, update the CRD's allowedNamespaces or restrictedNamespaces.\n\n",
					ns,
				),
				Code: 403,
			},
		})
		return
	}

	ws.namespaceStatsFor(gvrToKey(req.Resource)).RecordAllowed()
	_ = time.Since(start)

	ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
		UID: req.UID, Allowed: true,
	})
}

func (ws *WebhookServer) namespaceRulesForCRD(group, plural string) (*NamespaceRules, bool) {
	if ws.namespaceRuleMap == nil {
		return nil, false
	}
	key := plural + "." + group
	r, ok := ws.namespaceRuleMap[key]
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
