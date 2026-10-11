package shared

import (
	"encoding/json"
	"fmt"

	"github.com/inrundev/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ToObjectMap stamps apiVersion/kind onto obj and converts it to the
// map[string]interface{} form required by domain.Result.Resources and SSA apply.
// apiVersion is the full "group/version" string (e.g. "apps/v1", "v1").
func ToObjectMap(obj runtime.Object, apiVersion, kind string) (map[string]interface{}, error) {
	obj.GetObjectKind().SetGroupVersionKind(
		schema.FromAPIVersionAndKind(apiVersion, kind),
	)
	return runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
}

// DecodeFields JSON-round-trips a raw fields map into dst (must be a pointer).
// Used by Build functions to convert the intent fields map to a typed struct.
func DecodeFields(fields map[string]interface{}, dst interface{}) error {
	raw, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode fields: %w", err)
	}
	return nil
}

// ApplyMetaOverrides merges caller-supplied labels and annotations into an
// already-built object map. Called by every BuildFromIntent after ToObjectMap
// so that intent-form resources can carry arbitrary metadata without each
// builder duplicating the merge logic.
//
// Fails fast with a descriptive error if any key is not a valid qualified name
// or any label value is not a valid Kubernetes label value. Annotation values
// have no syntax restriction and are not validated.
func ApplyMetaOverrides(obj map[string]interface{}, lbls, annotations map[string]string) error {
	if len(lbls) == 0 && len(annotations) == 0 {
		return nil
	}
	if err := labels.ValidateLabelMap(lbls); err != nil {
		return err
	}
	if err := labels.ValidateAnnotationKeys(annotations); err != nil {
		return err
	}

	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		meta = make(map[string]interface{})
		obj["metadata"] = meta
	}
	if len(lbls) > 0 {
		existing, _ := meta["labels"].(map[string]interface{})
		if existing == nil {
			existing = make(map[string]interface{})
		}
		for k, v := range lbls {
			existing[k] = v
		}
		meta["labels"] = existing
	}
	if len(annotations) > 0 {
		existing, _ := meta["annotations"].(map[string]interface{})
		if existing == nil {
			existing = make(map[string]interface{})
		}
		for k, v := range annotations {
			existing[k] = v
		}
		meta["annotations"] = existing
	}
	return nil
}
