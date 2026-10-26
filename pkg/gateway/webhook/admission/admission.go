package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/util/kube"
)

// Validate serves /validate: it checks a CREATE or UPDATE against the CRD's
// validation rules and denies it with every failed rule.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	rv := Decode(w, r, "admission/validate")
	if rv == nil {
		return
	}

	req := rv.Request

	logger.Debug().
		Str("uid", req.UID).
		Str("kind", req.Kind.Kind).
		Str("name", req.Name).
		Str("namespace", req.Namespace).
		Str("operation", req.Operation).
		Msg("admission/validate: received request")

	resp := &Response{UID: req.UID, Allowed: true}

	gvrKey := kube.GVRKey(req.Resource.Group, req.Resource.Version, req.Resource.Resource)
	rules := h.registry.GetValidationRules(gvrKey)

	if rules == nil || len(rules.Rules) == 0 {
		Write(w, rv, resp)
		return
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(req.Object, &obj); err != nil {
		logger.Error().Err(err).
			Str("uid", req.UID).
			Str("kind", req.Kind.Kind).
			Msg("admission/validate: failed to unmarshal object")
		resp.Allowed = false
		resp.Status = &Status{
			Message: fmt.Sprintf("internal error: could not parse object: %v", err),
			Code:    500,
		}
		Write(w, rv, resp)
		return
	}

	denials, warnings := h.evaluateValidationRules(r.Context(), obj, rules, req.Kind.Kind)

	for _, w := range warnings {
		resp.Warnings = append(resp.Warnings,
			fmt.Sprintf("inrun: field %q: %s", w.Field, w.Message))
	}

	duration := time.Since(start)
	crdName := req.Kind.Kind

	admStats := h.stats.For(gvrKey)
	if len(denials) > 0 {
		admStats.RecordValidationDenied(duration)
		metrics.RecordValidationOutcome(crdName, "denied", metrics.MetricSourceAdmission)
		for _, d := range denials {
			metrics.RecordValidationViolation(crdName, d.Field, d.RuleType, "deny", metrics.MetricSourceAdmission)
		}
		for _, w := range warnings {
			metrics.RecordValidationViolation(crdName, w.Field, w.RuleType, "warn", metrics.MetricSourceAdmission)
		}
	} else if len(warnings) > 0 {
		admStats.RecordValidationWarned(duration)
		metrics.RecordValidationOutcome(crdName, "warned", metrics.MetricSourceAdmission)
		for _, w := range warnings {
			metrics.RecordValidationViolation(crdName, w.Field, w.RuleType, "warn", metrics.MetricSourceAdmission)
		}
	} else {
		admStats.RecordValidationAllowed(duration)
		metrics.RecordValidationOutcome(crdName, "allowed", metrics.MetricSourceAdmission)
	}
	metrics.RecordValidationDurationSeconds(crdName, duration.Seconds())

	if len(denials) > 0 {
		resp.Allowed = false
		msgs := make([]string, 0, len(denials))
		causes := make([]metav1.StatusCause, 0, len(denials))
		for _, d := range denials {
			msgs = append(msgs, fmt.Sprintf("field %q: %s (got: %q)", d.Field, d.Message, d.Got))
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   d.Field,
				Message: d.Message,
			})
		}
		resp.Status = &Status{
			Message: fmt.Sprintf(
				"\n\n[Inrun Validation] validation failed\n\n"+
					"%s/%s/%s was blocked due to the following policies:\n"+
					" %s\n\n", req.Kind.Kind, req.Name, req.Namespace, strings.Join(msgs, "; ")),
			Code:    400,
			Details: &metav1.StatusDetails{Causes: causes},
		}
		logger.Info().
			Str("kind", req.Kind.Kind).
			Str("name", req.Name).
			Str("namespace", req.Namespace).
			Str("uid", req.UID).
			Int("denials", len(denials)).
			Msg("admission/validate: rejected")
	} else if len(warnings) > 0 {
		logger.Info().
			Str("kind", req.Kind.Kind).
			Str("name", req.Name).
			Int("warnings", len(warnings)).
			Msg("admission/validate: allowed with warnings")
	} else {
		logger.Debug().
			Str("kind", req.Kind.Kind).
			Str("name", req.Name).
			Msg("admission/validate: allowed")
	}

	Write(w, rv, resp)
}

// Mutate serves /mutate: it applies the CRD's mutation rules and returns the
// changes as a JSON Patch. Mutation runs before validation, so validation sees
// the defaults.
func (h *Handler) Mutate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	rv := Decode(w, r, "admission/mutate")
	if rv == nil {
		return
	}

	req := rv.Request

	logger.Debug().
		Str("uid", req.UID).
		Str("kind", req.Kind.Kind).
		Str("name", req.Name).
		Str("namespace", req.Namespace).
		Str("operation", req.Operation).
		Msg("admission/mutate: received request")

	resp := &Response{UID: req.UID, Allowed: true}

	gvrKey := kube.GVRKey(req.Resource.Group, req.Resource.Version, req.Resource.Resource)
	rules := h.registry.GetMutationRules(gvrKey)

	if rules == nil || len(rules.Rules) == 0 {
		Write(w, rv, resp)
		return
	}

	var original map[string]interface{}
	if err := json.Unmarshal(req.Object, &original); err != nil {
		logger.Error().Err(err).
			Str("uid", req.UID).
			Str("kind", req.Kind.Kind).
			Msg("admission/mutate: failed to unmarshal object")
		Write(w, rv, resp)
		return
	}

	mutated := runtime.DeepCopyJSON(original)
	changes, err := h.applyMutationRules(context.Background(), mutated, rules, req.Kind.Kind)
	if err != nil {
		logger.Error().Err(err).
			Str("kind", req.Kind.Kind).
			Str("name", req.Name).
			Msg("admission/mutate: error applying rules — allowing without mutation")
		Write(w, rv, resp)
		return
	}

	crdName := req.Kind.Kind

	mutStats := h.stats.For(gvrKey)
	if len(changes) == 0 {
		duration := time.Since(start)
		mutStats.RecordMutationSkipped(duration)
		metrics.RecordMutationOutcome(crdName, "skipped", metrics.MetricSourceAdmission)
		metrics.RecordMutationDurationSeconds(crdName, duration.Seconds())
		logger.Debug().
			Str("kind", crdName).
			Str("name", req.Name).
			Msg("admission/mutate: no changes — allowed without patch")
		Write(w, rv, resp)
		return
	}

	patch, err := buildJSONPatch(original, mutated)
	if err != nil {
		logger.Error().Err(err).
			Str("kind", crdName).
			Str("name", req.Name).
			Msg("admission/mutate: failed to build patch — allowing without mutation")
		Write(w, rv, resp)
		return
	}

	resp.Patch = patch
	resp.PatchType = ptr.To("JSONPatch")

	duration := time.Since(start)
	mutStats.RecordMutationApplied(duration)
	metrics.RecordMutationOutcome(crdName, "applied", metrics.MetricSourceAdmission)
	for _, c := range changes {
		metrics.RecordMutationFieldApplied(crdName, c.Field, c.ChangeType, metrics.MetricSourceAdmission)
	}
	metrics.RecordMutationDurationSeconds(crdName, duration.Seconds())

	logger.Info().
		Str("kind", crdName).
		Str("name", req.Name).
		Int("changes", len(changes)).
		Msg("admission/mutate: defaults applied")

	Write(w, rv, resp)
}

// buildJSONPatch returns the JSON Patch (RFC 6902) that turns original into
// mutated. Diffing the two escapes keys such as example.com/owner and adds
// missing parents such as metadata.labels.
func buildJSONPatch(original, mutated map[string]interface{}) ([]byte, error) {
	from, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	to, err := json.Marshal(mutated)
	if err != nil {
		return nil, err
	}
	ops, err := jsonpatch.CreatePatch(from, to)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ops)
}
