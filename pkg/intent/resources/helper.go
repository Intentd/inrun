package resources

import (
	"encoding/json"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	kwtypes "github.com/intentd/kubewright/core/types"
)

// checkImage applies kubewright's rule for a pod kind's containers: image
// describes the one container, containers describe several — one of the two.
func checkImage(kind, image string, containers []kwtypes.Container) error {
	switch {
	case image == "" && len(containers) == 0:
		return fmt.Errorf("%s intent: image or containers is required", kind)
	case image != "" && len(containers) > 0:
		return fmt.Errorf("%s intent: set image or containers, not both", kind)
	}
	return nil
}

// envVars turns an intent's env map into env vars in key order, so the built
// object is the same on every reconcile. With containers, every container
// inherits them.
func envVars(env map[string]string) []kwtypes.EnvVar {
	if len(env) == 0 {
		return nil
	}
	names := make([]string, 0, len(env))
	for k := range env {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]kwtypes.EnvVar, 0, len(names))
	for _, k := range names {
		out = append(out, kwtypes.EnvVar{Name: k, Value: env[k]})
	}
	return out
}

// resolveNamespace is ns, or else owner's namespace, or else "default".
func resolveNamespace(owner domain.Object, ns string) string {
	if ns != "" {
		return ns
	}
	if owner.GetNamespace() != "" {
		return owner.GetNamespace()
	}
	return "default"
}

// toObjectMap stamps apiVersion/kind onto obj and converts it to the
// map[string]interface{} form required by domain.Result.Resources and SSA apply.
// apiVersion is the full "group/version" string (e.g. "apps/v1", "v1").
func toObjectMap(obj runtime.Object, apiVersion, kind string) (map[string]interface{}, error) {
	obj.GetObjectKind().SetGroupVersionKind(
		schema.FromAPIVersionAndKind(apiVersion, kind),
	)
	return runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
}

// decodeFields JSON-round-trips a raw fields map into dst (must be a pointer).
// Used by the builders to convert the intent fields map to a typed struct.
func decodeFields(fields map[string]interface{}, dst interface{}) error {
	raw, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode fields: %w", err)
	}
	return nil
}

// applyMetaOverrides merges caller-supplied labels and annotations into an
// already-built object map. Called by every builder after toObjectMap
// so that intent-form resources can carry arbitrary metadata without each
// builder duplicating the merge logic.
//
// Fails fast with a descriptive error if any key is not a valid qualified name
// or any label value is not a valid Kubernetes label value. Annotation values
// have no syntax restriction and are not validated.
func applyMetaOverrides(obj map[string]interface{}, lbls, annotations map[string]string) error {
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
