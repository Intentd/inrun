// webhook/conversion.go — /convert HTTP handler for CRD version conversion.
//
// The Kubernetes API server POSTs a ConversionReview to this endpoint when it
// needs to convert a CR from one version to another. Inrun applies the
// conversion rules declared in the Catalog for the object's Kind.
package webhook

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/metrics"
)

// --- Kubernetes ConversionReview types ---

type ConversionReview struct {
	APIVersion string                    `json:"apiVersion"`
	Kind       string                    `json:"kind"`
	Request    *ConversionReviewRequest  `json:"request,omitempty"`
	Response   *ConversionReviewResponse `json:"response,omitempty"`
}

type ConversionReviewRequest struct {
	UID               string            `json:"uid"`
	DesiredAPIVersion string            `json:"desiredAPIVersion"`
	Objects           []json.RawMessage `json:"objects"`
}

type ConversionReviewResponse struct {
	UID              string            `json:"uid"`
	ConvertedObjects []json.RawMessage `json:"convertedObjects"`
	Result           *Status           `json:"result"`
}

type Status struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (ws *WebhookServer) conversionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics.IncConversionRequests()
	defer metrics.DecConversionRequests()

	var review ConversionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		ws.conversionStatsFor("").RecordFailure() // kind unknown at decode time
		metrics.RecordConversionError("unknown", "invalid_request")
		http.Error(w, "invalid ConversionReview", http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		ws.conversionStatsFor("").RecordFailure()
		metrics.RecordConversionError("unknown", "missing_request")
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	sourceVersion := ""
	if len(review.Request.Objects) > 0 {
		var firstObj map[string]interface{}
		if err := json.Unmarshal(review.Request.Objects[0], &firstObj); err == nil {
			if apiVersion, ok := firstObj["apiVersion"].(string); ok {
				if parts := strings.Split(apiVersion, "/"); len(parts) == 2 {
					sourceVersion = parts[1]
				}
			}
		}
	}

	targetVersion := ""
	if desired := review.Request.DesiredAPIVersion; desired != "" {
		if parts := strings.Split(desired, "/"); len(parts) == 2 {
			targetVersion = parts[1]
		}
	}

	resp := &ConversionReviewResponse{
		UID:              review.Request.UID,
		ConvertedObjects: make([]json.RawMessage, len(review.Request.Objects)),
		Result:           &Status{Status: "Success"},
	}

	var kind string

	for i, raw := range review.Request.Objects {
		objStart := time.Now()

		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			resp.Result = &Status{Status: "Failure", Message: "invalid object payload"}
			metrics.RecordConversionError(kind, "invalid_payload")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			// Object is undecodable — no GVK to derive a stats key from.
			// Record under "" so failures are visible without fabricating CRD attribution.
			ws.conversionStatsFor("").RecordFailure()
			break
		}

		kind, _ = obj["kind"].(string)
		if kind == "" {
			resp.Result = &Status{Status: "Failure", Message: "object missing kind"}
			metrics.RecordConversionError(kind, "missing_kind")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			// Object decoded but carries no kind field — same unattributable case.
			ws.conversionStatsFor("").RecordFailure()
			break
		}

		// Build the GVK key from the object's own apiVersion+kind so that
		// multi-version CRDs sharing the same kind (e.g. v1/CronJob and
		// v2/CronJob) each get their own stats bucket rather than colliding.
		objAPIVersion, _ := obj["apiVersion"].(string)
		var objGroup, objVersion string
		if parts := strings.SplitN(objAPIVersion, "/", 2); len(parts) == 2 {
			objGroup, objVersion = parts[0], parts[1]
		} else {
			objVersion = objAPIVersion // core group: no slash
		}
		gvkKey := crdGVKKey(objGroup, objVersion, kind)

		rules := ws.conversionRegistry.GetConversionRules(kind)
		if rules == nil {
			resp.Result = &Status{Status: "Failure", Message: "no conversion rules for kind"}
			metrics.RecordConversionError(kind, "no_rules")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			ws.conversionStatsFor(ws.gvkToGVRKey[gvkKey]).RecordFailure()
			break
		}

		converted, err := applyConversion(obj, rules, review.Request.DesiredAPIVersion)
		if err != nil {
			resp.Result = &Status{Status: "Failure", Message: err.Error()}
			metrics.RecordConversionError(kind, "apply_failed")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			ws.conversionStatsFor(ws.gvkToGVRKey[gvkKey]).RecordFailure()
			break
		}

		objDuration := time.Since(objStart).Seconds()
		metrics.ObserveConversionDuration(kind, sourceVersion, targetVersion, objDuration)
		metrics.RecordConversion(kind, sourceVersion, targetVersion, "success")
		ws.conversionStatsFor(ws.gvkToGVRKey[gvkKey]).RecordSuccess(time.Duration(objDuration * float64(time.Second)))

		out, _ := json.Marshal(converted)
		resp.ConvertedObjects[i] = out
	}

	response := ConversionReview{
		APIVersion: "apiextensions.k8s.io/v1",
		Kind:       "ConversionReview",
		Response:   resp,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// processConversionForTest exposes the conversion logic for unit tests.
func processConversionForTest(review ConversionReview, rulesRegistry catalog.ConversionRegistry) ConversionReview {
	resp := &ConversionReviewResponse{
		UID:              review.Request.UID,
		ConvertedObjects: make([]json.RawMessage, len(review.Request.Objects)),
		Result:           &Status{Status: "Success"},
	}

	for i, raw := range review.Request.Objects {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			resp.Result = &Status{Status: "Failure", Message: "invalid object payload"}
			break
		}
		kind, _ := obj["kind"].(string)
		if kind == "" {
			resp.Result = &Status{Status: "Failure", Message: "object missing kind"}
			break
		}
		rules := rulesRegistry.GetConversionRules(kind)
		if rules == nil {
			resp.Result = &Status{Status: "Failure", Message: "no conversion rules for kind"}
			break
		}
		converted, err := applyConversion(obj, rules, review.Request.DesiredAPIVersion)
		if err != nil {
			resp.Result = &Status{Status: "Failure", Message: err.Error()}
			break
		}
		out, _ := json.Marshal(converted)
		resp.ConvertedObjects[i] = out
	}

	return ConversionReview{
		APIVersion: "apiextensions.k8s.io/v1",
		Kind:       "ConversionReview",
		Response:   resp,
	}
}
