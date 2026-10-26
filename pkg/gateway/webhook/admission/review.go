package admission

import (
	"encoding/json"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/logger"
)

// The AdmissionReview the API server sends to the admission and protection
// webhooks. The types mirror admission.k8s.io/v1, declared here so the
// handlers need no more of k8s.io/api.

// Review is the AdmissionReview: a Request from the API server, and the
// Response the gateway sends back in the same envelope.
type Review struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Request    *Request  `json:"request,omitempty"`
	Response   *Response `json:"response,omitempty"`
}

// Request is the object the API server asks about.
type Request struct {
	UID       string                      `json:"uid"`
	Kind      metav1.GroupVersionKind     `json:"kind"`
	Resource  metav1.GroupVersionResource `json:"resource"`
	Name      string                      `json:"name,omitempty"`
	Namespace string                      `json:"namespace,omitempty"`
	Operation string                      `json:"operation"`
	Object    json.RawMessage             `json:"object,omitempty"`
	OldObject json.RawMessage             `json:"oldObject,omitempty"`
	DryRun    *bool                       `json:"dryRun,omitempty"`
}

// Response is the gateway's answer to a Request.
type Response struct {
	UID       string   `json:"uid"`
	Allowed   bool     `json:"allowed"`
	Status    *Status  `json:"status,omitempty"`
	Patch     []byte   `json:"patch,omitempty"`
	PatchType *string  `json:"patchType,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Status is why a Request was denied.
//
// Message is one self-contained string, because kubectl apply prints it as
// is. Details.Causes carries the same denials per field: the API server
// passes them on in metav1.Status.Details, which is how the Gateway API's
// ApplyResponse.Violations gets them for the console's form.
type Status struct {
	Message string                `json:"message"`
	Code    int32                 `json:"code"`
	Details *metav1.StatusDetails `json:"details,omitempty"`
}

// Decode reads the Review a webhook was POSTed. When the request is not a
// Review with a Request, it answers with the error itself and returns nil;
// handler names the webhook in the log.
func Decode(w http.ResponseWriter, r *http.Request, handler string) *Review {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil
	}
	var rv Review
	if err := json.NewDecoder(r.Body).Decode(&rv); err != nil {
		logger.Error().Err(err).Msgf("%s: failed to decode AdmissionReview", handler)
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return nil
	}
	if rv.Request == nil {
		http.Error(w, "missing request in AdmissionReview", http.StatusBadRequest)
		return nil
	}
	return &rv
}

// Write sends resp in reply to rv.
func Write(w http.ResponseWriter, rv *Review, resp *Response) {
	out := Review{APIVersion: rv.APIVersion, Kind: rv.Kind, Response: resp}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		logger.Error().Err(err).Str("uid", resp.UID).Msg("admission: failed to encode response")
	}
}

// Allow allows rv's request.
func Allow(w http.ResponseWriter, rv *Review) {
	Write(w, rv, &Response{UID: rv.Request.UID, Allowed: true})
}

// Deny denies rv's request as forbidden, with message for kubectl to print.
func Deny(w http.ResponseWriter, rv *Review, message string) {
	Write(w, rv, &Response{
		UID:     rv.Request.UID,
		Allowed: false,
		Status:  &Status{Message: message, Code: http.StatusForbidden},
	})
}
