package labels

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/validate/content"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateLabelMap checks every entry in labels as a Kubernetes label:
// both the key and the value must be valid.
func ValidateLabelMap(labels map[string]string) error {
	for k, v := range labels {
		if errs := content.IsLabelKey(k); len(errs) > 0 {
			return fmt.Errorf("invalid label key %q: %s", k, strings.Join(errs, "; "))
		}
		if errs := content.IsLabelValue(v); len(errs) > 0 {
			return fmt.Errorf("invalid label value for key %q: %s", k, strings.Join(errs, "; "))
		}
	}
	return nil
}

// ValidateAnnotationKeys checks annotation keys against the label-key rules,
// which Kubernetes also applies to annotation keys. Values are intentionally
// not checked: unlike labels, annotation values are unconstrained strings.
func ValidateAnnotationKeys(annotations map[string]string) error {
	for k := range annotations {
		if errs := content.IsLabelKey(k); len(errs) > 0 {
			return fmt.Errorf("invalid annotation key %q: %s", k, strings.Join(errs, "; "))
		}
	}
	return nil
}

// ValidateLabelSelector reports whether sel is a syntactically valid
// Kubernetes label selector (e.g. "app=nginx", "tier in (gold,silver)").
// The parsed selector is discarded — callers only need to know whether the
// string would be accepted by the API server.
func ValidateLabelSelector(sel string) (bool, error) {
	if _, err := metav1.ParseToLabelSelector(sel); err != nil {
		return false, err
	}
	return true, nil
}

// IsValidLabelSelector reports whether sel is a syntactically valid
// Kubernetes label selector (e.g. "app=nginx", "tier in (gold,silver)").
// It is the boolean-only form of ValidateLabelSelector: parse errors are
// collapsed into false, so callers that need to report why a selector is
// invalid should call ValidateLabelSelector instead.
func IsValidLabelSelector(sel string) bool {
	_, err := metav1.ParseToLabelSelector(sel)
	return err == nil
}
