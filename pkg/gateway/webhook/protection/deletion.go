package protection

import (
	"fmt"
	"net/http"

	"github.com/intentd/inrun/pkg/gateway/webhook/admission"
	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// Deletion serves /deletion-protection: it blocks DELETE on the protected
// CRDs and on Inrun's own resources (deployment, service, ingress, webhook
// configurations). The webhook's failurePolicy is Fail, so DELETE is blocked
// while the gateway is unreachable. To decommission, set
// security.deletionProtection.enabled: false, redeploy, then delete.
func (h *Handler) Deletion(w http.ResponseWriter, r *http.Request) {
	rv := admission.Decode(w, r, deletionProtection)
	if rv == nil {
		return
	}

	req := rv.Request

	if req.Operation != "DELETE" {
		admission.Allow(w, rv)
		return
	}

	// Self-protection: block deletion of the deletion-protection webhook itself.
	if req.Kind.Kind == "ValidatingWebhookConfiguration" &&
		req.Name == registration.DeletionProtectionConfigName {

		logger.Info().
			Str("webhook", req.Name).
			Str("uid", req.UID).
			Msgf("%s: blocking deletion of the deletion-protection webhook", deletionProtection)

		metrics.RecordDeletionProtectionBlocked(registration.DeletionProtectionConfigName)
		h.infraStats.RecordBlocked() // infra event — not attributable to a specific CRD

		admission.Deny(w, rv, fmt.Sprintf(
			"\n\n[Inrun Security] The deletion-protection webhook \"%s\" is itself protected.\n\n"+
				"To disable deletion protection entirely:\n"+
				"- Set security.deletionProtection.enabled: false in the Blueprint\n"+
				"- Redeploy Inrun Gateway, then delete the webhook.\n\n",
			registration.DeletionProtectionConfigName,
		))
		return
	}

	isCRD := req.Resource.Group == "apiextensions.k8s.io" &&
		req.Resource.Resource == "customresourcedefinitions"

	if isCRD {
		// req.Name is the full CRD name ("plural.group", e.g. "websites.demo.io").
		// Look up the per-CRD stats instance via the reverse-lookup table.
		crdStats := h.deletionStats.For(h.crdGVRKeys[req.Name])
		if h.isProtectedCRD(req.Name) {
			logger.Info().
				Str("crd", req.Name).
				Str("uid", req.UID).
				Msgf("%s: blocking CRD deletion", deletionProtection)

			metrics.RecordDeletionProtectionBlocked(req.Name)
			crdStats.RecordBlocked()

			admission.Deny(w, rv, fmt.Sprintf(
				"\n\n[Inrun Security] CRD %q is protected from deletion.\n\n"+
					"To delete it:\n"+
					"- Set security.deletionProtection.enabled: false in the Blueprint\n"+
					"- Redeploy Inrun Gateway, then delete the CRD.\n\n",
				req.Name,
			))
			return
		}
		crdStats.RecordAllowed()
		admission.Allow(w, rv)
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
	h.infraStats.RecordBlocked()

	kind := req.Kind.Kind
	name := req.Name
	ns := req.Namespace

	footer := "\n\nTo remove protection:\n" +
		"- Set security.deletionProtection.enabled: false in the Blueprint.\n" +
		"- Redeploy Inrun Gateway.\n" +
		"- Retry the deletion."

	var header string
	if ns == "" || kind == "Namespace" {
		header = fmt.Sprintf("The %s %q is protected from deletion.", kind, name)
	} else {
		header = fmt.Sprintf("The %s %q in namespace %q is protected from deletion.", kind, name, ns)
	}

	message := "\n\n[Inrun Security] " + header + footer

	admission.Deny(w, rv, message)
}

func (h *Handler) isProtectedCRD(crdName string) bool {
	if h.protectedCRDs == nil {
		return false
	}
	_, ok := h.protectedCRDs[crdName]
	return ok
}
