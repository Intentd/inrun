package intent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/intentd/inrun/pkg/labels"
)

// Target extracts the effective target from a CR's annotations.
// Resolution order:
//  1. serve-alias annotation (most specific)
//  2. serve-target annotation (primary target)
//  3. Empty string (no target found)
func Target(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}

	// 1. Check alias first (most specific)
	if alias, ok := annotations[labels.AnnotationServeAlias]; ok && alias != "" {
		return alias
	}

	// 2. Fall back to target
	if target, ok := annotations[labels.AnnotationServeTarget]; ok && target != "" {
		return target
	}

	return ""
}

// SetTarget marks obj as created through target, the way the gateway does.
// The runtime picks a CR's target from this annotation only (see Target);
// without it a CR is a direct apply and reconciles with the CRD-level box.
// An empty target leaves obj unchanged. This is used by simulate
func SetTarget(obj *unstructured.Unstructured, target string) {
	if target == "" {
		return
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[labels.AnnotationServeTarget] = target
	obj.SetAnnotations(ann)
}

// FromObject extracts the raw intent payload from the
// inrun.dev/serve-intent annotation on a CR object map.
// Returns nil when the annotation is absent or unparseable.
// Used by both the webhook and the reconciler to inject .request into
// the resolver so validation rules can reference intent-vocabulary fields.
func FromObject(obj map[string]interface{}) map[string]interface{} {
	return labels.ResolveAnnotationFromObject(obj, labels.AnnotationServeIntent)
}

// Decode parses an intent written as JSON or YAML. YAML is converted to JSON
// first, so values have json.Unmarshal's types (numbers are float64) whatever
// the format, and the same intent builds the same CR either way.
func Decode(data []byte) (map[string]interface{}, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON or YAML: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("empty intent")
	}
	return raw, nil
}
