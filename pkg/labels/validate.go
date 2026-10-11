package labels

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/validate/content"
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
