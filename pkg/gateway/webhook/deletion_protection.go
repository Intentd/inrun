// webhook/deletion_protection.go — /deletion-protection webhook handler.
//
// Registered only when security.deletionProtection.enabled: true in the Catalog.
// Intercepts DELETE operations on CRDs owned by this operator and Inrun's
// own resources (deployment, service, ingress, webhook configurations).
//
// failurePolicy: Fail — if Inrun is unreachable, DELETE is blocked.
// To decommission: set deletionProtection.enabled: false, redeploy, then delete.
package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

func (ws *WebhookServer) deletionProtectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	var review AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logger.Error().Err(err).Msgf("%s: failed to decode AdmissionReview", deletionProtection)
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	req := review.Request

	if req.Operation != "DELETE" {
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
		return
	}

	// Self-protection: block deletion of the deletion-protection webhook itself.
	if req.Kind.Kind == "ValidatingWebhookConfiguration" &&
		req.Name == deletionProtectionWebhookConfigName {

		logger.Info().
			Str("webhook", req.Name).
			Str("uid", req.UID).
			Msgf("%s: blocking deletion of the deletion-protection webhook", deletionProtection)

		metrics.RecordDeletionProtectionBlocked(deletionProtectionWebhookConfigName)
		ws.infraProtStats.RecordBlocked() // infra event — not attributable to a specific CRD

		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Status: &AdmissionStatus{
				Message: fmt.Sprintf(
					"\n\n[Inrun Security] The deletion-protection webhook \"%s\" is itself protected.\n\n"+
						"To disable deletion protection entirely:\n"+
						"- Set security.deletionProtection.enabled: false in the Catalog\n"+
						"- Redeploy Inrun Gateway, then delete the webhook.\n\n",
					deletionProtectionWebhookConfigName,
				),
				Code: 403,
			},
		})
		return
	}

	isCRD := req.Resource.Group == "apiextensions.k8s.io" &&
		req.Resource.Resource == "customresourcedefinitions"

	if isCRD {
		// req.Name is the full CRD name ("plural.group", e.g. "websites.demo.io").
		// Look up the per-CRD stats instance via the reverse-lookup table.
		crdStats := ws.protectionStatsFor(ws.crdNameToGVRKey[req.Name])
		if ws.isProtectedCRD(req.Name) {
			logger.Info().
				Str("crd", req.Name).
				Str("uid", req.UID).
				Msgf("%s: blocking CRD deletion", deletionProtection)

			metrics.RecordDeletionProtectionBlocked(req.Name)
			crdStats.RecordBlocked()
			_ = time.Since(start)

			ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
				UID:     req.UID,
				Allowed: false,
				Status: &AdmissionStatus{
					Message: fmt.Sprintf(
						"\n\n[Inrun Security] CRD %q is protected from deletion.\n\n"+
							"To delete it:\n"+
							"- Set security.deletionProtection.enabled: false in the Catalog\n"+
							"- Redeploy Inrun Gateway, then delete the CRD.\n\n",
						req.Name,
					),
					Code: 403,
				},
			})
			return
		}
		crdStats.RecordAllowed()
		ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
			UID: req.UID, Allowed: true,
		})
		return
	}

	// Non-CRD Inrun resource (Deployment, Service, etc.) — always block.
	// Counted against infraProtStats because these have no CRD GVR.
	logger.Info().
		Str("resource", req.Resource.Resource).
		Str("name", req.Name).
		Str("namespace", req.Namespace).
		Str("uid", req.UID).
		Msgf("%s: blocking Inrun resource deletion", deletionProtection)

	metrics.RecordDeletionProtectionBlocked("inrun-" + req.Resource.Resource)
	ws.infraProtStats.RecordBlocked()

	kind := req.Kind.Kind
	name := req.Name
	ns := req.Namespace

	footer := "\n\nTo remove protection:\n" +
		"- Set security.deletionProtection.enabled: false in the Catalog.\n" +
		"- Redeploy Inrun Gateway.\n" +
		"- Retry the deletion."

	var header string
	if ns == "" || kind == "Namespace" {
		header = fmt.Sprintf("The %s %q is protected from deletion.", kind, name)
	} else {
		header = fmt.Sprintf("The %s %q in namespace %q is protected from deletion.", kind, name, ns)
	}

	message := "\n\n[Inrun Security] " + header + footer

	ws.writeAdmissionResponse(w, review.APIVersion, review.Kind, &AdmissionResponse{
		UID:     req.UID,
		Allowed: false,
		Status: &AdmissionStatus{
			Message: message,
			Code:    403,
		},
	})
}

func (ws *WebhookServer) isProtectedCRD(crdName string) bool {
	if ws.protectedCRDNames == nil {
		return false
	}
	_, ok := ws.protectedCRDNames[crdName]
	return ok
}
