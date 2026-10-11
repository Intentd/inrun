// webhook/strict_mode_protection.go — /strict-mode-protection webhook handler.
//
// Registered only when security.deletionProtection.strictMode: true in the Catalog.
// Intercepts UPDATE operations on resources that carry the deletion-protection label
// and denies any request that removes that label — treating label removal as a
// deletion attempt.
//
// # Design
//
// This handler is intentionally separate from /deletion-protection. Deletion
// protection guards DELETE operations. Strict mode guards label removal on UPDATE.
// Keeping them separate keeps each handler focused and independently controllable.
//
// # Enforcement
//
// The webhook ObjectSelector is set to the deletion-protection label selector, so
// the API server only calls this handler when either the old or new object carries
// the label. The handler then checks:
//
//	oldObject has label AND newObject does not → DENY (label removal blocked)
//	otherwise → ALLOW
//
// Enforcement is stateless: every decision is made by comparing old and new labels
// in the admission request. No in-process register is required.
//
// # Unlock
//
// To remove the label from a protected resource, set strictMode: false in the
// Catalog, edit the Inrun ConfigMap, and restart Inrun. The new pods will
// not register the strict-mode webhook, so label removal becomes possible again.
// This mirrors how deletion protection itself is disabled — the trust boundary is
// the Catalog, not a separate escape hatch.
package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// strictModeProtectionHandler handles /strict-mode-protection admission reviews.
// Denies UPDATE requests that remove the deletion-protection label.
func (ws *WebhookServer) strictModeProtectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var review AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logger.Error().Err(err).Msgf("%s: failed to decode AdmissionReview", strictModeProtection)
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	req := review.Request

	if req.Operation != "UPDATE" {
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
		return
	}

	oldLabels := extractLabels(req.OldObject)
	newLabels := extractLabels(req.Object)

	hadLabel := oldLabels[labels.DeletionProtectionLabel] == "true"
	hasLabel := newLabels[labels.DeletionProtectionLabel] == "true"

	// --- EXEMPTION CHECK ---
	if newLabels[labels.StrictModeExemptKey] == "true" {
		// Exempt from strict mode – allow label removal
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
		return
	}

	if hadLabel && !hasLabel {
		kind := req.Kind.Kind
		name := req.Name
		ns := req.Namespace

		logger.Warn().
			Str("kind", kind).
			Str("name", name).
			Str("namespace", ns).
			Str("uid", req.UID).
			Msgf("%s: blocking deletion-protection label removal", strictModeProtection)

		metrics.RecordDeletionProtectionBlocked("strict-mode:" + kind)
		ws.strictModeStats.RecordBlocked()

		var header string
		if ns == "" {
			header = fmt.Sprintf("The %s %q carries the deletion-protection label.", kind, name)
		} else {
			header = fmt.Sprintf("The %s %q in namespace %q carries the deletion-protection label.", kind, name, ns)
		}

		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Status: &AdmissionStatus{
				Message: fmt.Sprintf(
					"\n\n[Inrun Security] %s\n\n"+
						"Removing this label is blocked because strictMode is enabled.\n\n"+
						"To unprotect this resource:\n"+
						"- Opt out in the catalog using: '<crd>.deletionProtection.strictMode: false'\n\n"+

						"To disable strict mode globally:\n"+
						"- Set security.deletionProtection.strictMode: false in the Catalog\n"+
						"- Regenerate and apply the bundle, and restart the Gateway\n\n",
					header,
				),
				Code: 403,
			},
		})
		return
	}

	ws.strictModeStats.RecordAllowed()
	ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
		UID: req.UID, Allowed: true,
	})
}

// extractLabels reads the labels map from a raw JSON object.
// Returns nil on any error — the handler treats missing labels as absent.
func extractLabels(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var obj struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj.Metadata.Labels
}
