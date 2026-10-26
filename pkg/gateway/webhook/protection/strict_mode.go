package protection

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/intentd/inrun/pkg/gateway/webhook/admission"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// StrictMode serves /strict-mode-protection: it denies an UPDATE that removes
// the deletion-protection label, treating the removal as a deletion attempt.
//
// This handler is intentionally separate from /deletion-protection. Deletion
// protection guards DELETE operations. Strict mode guards label removal on UPDATE.
// Keeping them separate keeps each handler focused and independently controllable.
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
// To remove the label from a protected resource, set strictMode: false in the
// Blueprint, edit the Inrun ConfigMap, and restart Inrun. The new pods will
// not register the strict-mode webhook, so label removal becomes possible again.
// This mirrors how deletion protection itself is disabled — the trust boundary is
// the Blueprint, not a separate escape hatch.
func (h *Handler) StrictMode(w http.ResponseWriter, r *http.Request) {
	rv := admission.Decode(w, r, strictModeProtection)
	if rv == nil {
		return
	}

	req := rv.Request

	if req.Operation != "UPDATE" {
		admission.Allow(w, rv)
		return
	}

	oldLabels := extractLabels(req.OldObject)
	newLabels := extractLabels(req.Object)

	hadLabel := oldLabels[labels.DeletionProtectionLabel] == "true"
	hasLabel := newLabels[labels.DeletionProtectionLabel] == "true"

	// --- EXEMPTION CHECK ---
	if newLabels[labels.StrictModeExemptKey] == "true" {
		// Exempt from strict mode – allow label removal
		admission.Allow(w, rv)
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
		h.strictModeStats.RecordBlocked()

		var header string
		if ns == "" {
			header = fmt.Sprintf("The %s %q carries the deletion-protection label.", kind, name)
		} else {
			header = fmt.Sprintf("The %s %q in namespace %q carries the deletion-protection label.", kind, name, ns)
		}

		admission.Deny(w, rv, fmt.Sprintf(
			"\n\n[Inrun Security] %s\n\n"+
				"Removing this label is blocked because strictMode is enabled.\n\n"+
				"To unprotect this resource:\n"+
				"- Opt out in the blueprint using: '<crd>.deletionProtection.strictMode: false'\n\n"+

				"To disable strict mode globally:\n"+
				"- Set security.deletionProtection.strictMode: false in the Blueprint\n"+
				"- Regenerate and apply the bundle, and restart the Gateway\n\n",
			header,
		))
		return
	}

	h.strictModeStats.RecordAllowed()
	admission.Allow(w, rv)
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
