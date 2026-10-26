package labels

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func InjectAnnotationToObject(obj *unstructured.Unstructured, raw map[string]interface{}, annotation string) {
	b, err := json.Marshal(raw)
	if err != nil {
		return
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[annotation] = string(b)
	obj.SetAnnotations(ann)
}

func ResolveAnnotationFromObject(obj map[string]interface{}, annotation string) map[string]interface{} {
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	annotations, ok := meta["annotations"].(map[string]interface{})
	if !ok {
		return nil
	}
	annJSON, ok := annotations[annotation].(string)
	if !ok || annJSON == "" {
		return nil
	}
	var ann map[string]interface{}
	if err := json.Unmarshal([]byte(annJSON), &ann); err != nil {
		return nil
	}
	return ann
}

// ResolveResourceMetricFromObject extracts the raw metrics payload from the
// inrun.dev/cross-metric annotation on a CR object map.
// This annotation is stamped by the runtime
// Returns nil when the annotation is absent or unparseable.
// Used by both the gateway to inject .metrics into
// the resolver so validation rules can reference metric-vocabulary fields.
func ResolveResourceMetricFromObject(obj map[string]interface{}) map[string]interface{} {
	return ResolveAnnotationFromObject(obj, AnnotationCrossMetrics)
}

// ResolveResourceHealthFromObject extracts the raw health payload from the
// inrun.dev/health annotation on a CR object map.
// This annotation is stamped by the runtime
// Returns nil when the annotation is absent or unparseable.
// Used by both the gateway to inject .health into
// the resolver so validation rules can reference health-vocabulary fields.
func ResolveResourceHealthFromObject(obj map[string]interface{}) map[string]interface{} {
	return ResolveAnnotationFromObject(obj, AnnotationHealth)
}

// InjectMetricsAnnotation stores the raw metrics payload as a JSON-encoded
// annotation so the admission webhook can bind it as .metrics in validation
// rules, enabling metrics-level gates.
func InjectMetricsAnnotation(obj *unstructured.Unstructured, raw map[string]interface{}) {
	InjectAnnotationToObject(obj, raw, AnnotationCrossMetrics)
}

// InjectHealthAnnotation stores the raw health payload as a JSON-encoded
// annotation so the admission webhook can bind it as .health in validation
// rules, enabling health-level gates.
func InjectHealthAnnotation(obj *unstructured.Unstructured, raw map[string]interface{}) {
	InjectAnnotationToObject(obj, raw, AnnotationHealth)
}
