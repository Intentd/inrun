package conversion

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/util/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConversionReview mirrors apiextensions.k8s.io/v1 ConversionReview.
type ConversionReview struct {
	APIVersion string                    `json:"apiVersion"`
	Kind       string                    `json:"kind"`
	Request    *ConversionReviewRequest  `json:"request,omitempty"`
	Response   *ConversionReviewResponse `json:"response,omitempty"`
}

// ConversionReviewRequest is the objects to convert and the version to convert them to.
type ConversionReviewRequest struct {
	UID               string            `json:"uid"`
	DesiredAPIVersion string            `json:"desiredAPIVersion"`
	Objects           []json.RawMessage `json:"objects"`
}

// ConversionReviewResponse is the converted objects, in the order received.
type ConversionReviewResponse struct {
	UID              string            `json:"uid"`
	ConvertedObjects []json.RawMessage `json:"convertedObjects"`
	Result           *Status           `json:"result"`
}

// Status is Success, or Failure with a message.
type Status struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Handler serves /convert and counts conversions per CRD.
type Handler struct {
	registry blueprint.ConversionRegistry
	// gvrKeys maps an object's kube.GVKKey to its CRD's kube.GVRKey, for its
	// stats; each version of a multi-version CRD has its own entry.
	gvrKeys map[string]string
	stats   *health.StatsMap[health.ConversionStats]
}

// New builds the Handler for the Blueprint's conversion rules. window is how
// many recent conversions the latency stats cover.
func New(bp *blueprint.Blueprint, window int) *Handler {
	h := &Handler{
		registry: bp.ConversionRegistry(),
		gvrKeys:  map[string]string{},
		stats: health.NewStatsMap(func() *health.ConversionStats {
			return health.NewConversionStats(window)
		}),
	}
	for _, crd := range bp.Enabled() {
		gvr := crd.GVR()
		key := kube.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		h.stats.For(key)
		h.gvrKeys[kube.GVKKey(crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind)] = key
	}
	return h
}

// StatsFor returns the conversion stats for gvrKey, or nil.
func (h *Handler) StatsFor(gvrKey string) *health.ConversionStats {
	return h.stats.Get(gvrKey)
}

// Convert serves /convert: the API server POSTs a ConversionReview when it
// needs a CR at another version, and Convert applies the Blueprint's
// conversion rules for the object's kind.
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics.IncConversionRequests()
	defer metrics.DecConversionRequests()

	var review ConversionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		h.stats.For("").RecordFailure() // kind unknown at decode time
		metrics.RecordConversionError("unknown", "invalid_request")
		http.Error(w, "invalid ConversionReview", http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		h.stats.For("").RecordFailure()
		metrics.RecordConversionError("unknown", "missing_request")
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	sourceVersion := ""
	if len(review.Request.Objects) > 0 {
		var firstObj map[string]interface{}
		if err := json.Unmarshal(review.Request.Objects[0], &firstObj); err == nil {
			if apiVersion, ok := firstObj["apiVersion"].(string); ok {
				if gv, err := schema.ParseGroupVersion(apiVersion); err == nil {
					sourceVersion = gv.Version
				}
			}
		}
	}

	targetVersion := ""
	if desired := review.Request.DesiredAPIVersion; desired != "" {
		if gv, err := schema.ParseGroupVersion(desired); err == nil {
			targetVersion = gv.Version
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
			h.stats.For("").RecordFailure()
			break
		}

		kind, _ = obj["kind"].(string)
		if kind == "" {
			resp.Result = &Status{Status: "Failure", Message: "object missing kind"}
			metrics.RecordConversionError(kind, "missing_kind")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			// Object decoded but carries no kind field — same unattributable case.
			h.stats.For("").RecordFailure()
			break
		}

		// Build the GVK key from the object's own apiVersion+kind so that
		// multi-version CRDs sharing the same kind (e.g. v1/CronJob and
		// v2/CronJob) each get their own stats bucket rather than colliding.
		objAPIVersion, _ := obj["apiVersion"].(string)
		gvk, err := kube.GVKFromFields(objAPIVersion, kind)
		if err != nil {
			resp.Result = &Status{Status: "Failure", Message: err.Error()}
			metrics.RecordConversionError(kind, "invalid_api_version")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			h.stats.For("").RecordFailure()
			break
		}
		gvkKey := kube.GVKKey(gvk.Group, gvk.Version, gvk.Kind)

		rules := h.registry.GetConversionRules(kind)
		if rules == nil {
			resp.Result = &Status{Status: "Failure", Message: "no conversion rules for kind"}
			metrics.RecordConversionError(kind, "no_rules")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			h.stats.For(h.gvrKeys[gvkKey]).RecordFailure()
			break
		}

		converted, err := applyConversion(obj, rules, review.Request.DesiredAPIVersion)
		if err != nil {
			resp.Result = &Status{Status: "Failure", Message: err.Error()}
			metrics.RecordConversionError(kind, "apply_failed")
			metrics.RecordConversion(kind, sourceVersion, targetVersion, "failure")
			h.stats.For(h.gvrKeys[gvkKey]).RecordFailure()
			break
		}

		objDuration := time.Since(objStart).Seconds()
		metrics.ObserveConversionDuration(kind, sourceVersion, targetVersion, objDuration)
		metrics.RecordConversion(kind, sourceVersion, targetVersion, "success")
		h.stats.For(h.gvrKeys[gvkKey]).RecordSuccess(time.Duration(objDuration * float64(time.Second)))

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
