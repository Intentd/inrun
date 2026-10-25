package kind

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// kindSnapshot is the recovery record written to the destination CR's
// AnnotationMigrateKindSnapshot. It captures everything rollback needs to
// undo a CR-level migration: the source's identity, metadata, and spec (to
// recreate it if it was pruned), the destination's identity (to delete it),
// and whether the source was pruned (to know whether recreation is needed).
type kindSnapshot struct {
	Source      kindSnapshotSource `json:"source"`
	Destination kindSnapshotTarget `json:"destination"`
	Pruned      bool               `json:"pruned"`
}

type kindSnapshotSource struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   kindSnapshotSourceMeta `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	Status     map[string]interface{} `json:"status,omitempty"`
}

type kindSnapshotSourceMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type kindSnapshotTarget struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// crSnapshot builds the recovery record for a CR-level migration. It captures
// the source's identity, labels, annotations, and spec — everything needed to
// recreate the source if rollback runs after prune. The spec and metadata are
// deep-copied so later mutations of the source do not alter the snapshot.
// Tooling annotations (kubectl.kubernetes.io/*, inrun.dev/*) are omitted:
// they describe how the source was applied, not what it declared, and
// restoring them onto a recreated source would be misleading.
func crSnapshot(src, dst *unstructured.Unstructured, prune bool) ([]byte, error) {
	snap := kindSnapshot{
		Source: kindSnapshotSource{
			APIVersion: src.GetAPIVersion(),
			Kind:       src.GetKind(),
			Metadata: kindSnapshotSourceMeta{
				Name:        src.GetName(),
				Namespace:   src.GetNamespace(),
				Labels:      copyStringMap(src.GetLabels()),
				Annotations: copyStringMap(filterToolingAnnotations(src.GetAnnotations())),
			},
			Spec:   copySpec(src.Object["spec"]),
			Status: copySpec(src.Object["status"]),
		},
		Destination: kindSnapshotTarget{
			APIVersion: dst.GetAPIVersion(),
			Kind:       dst.GetKind(),
		},
		Pruned: prune,
	}
	snap.Destination.Metadata.Name = dst.GetName()
	snap.Destination.Metadata.Namespace = dst.GetNamespace()
	return json.Marshal(snap)
}

// filterToolingAnnotations removes annotations written by tooling rather than
// by the user: kubectl's last-applied bookkeeping and Inrun's provenance
// stamps. Restoring these onto a recreated source would claim a provenance
// the rollback did not produce. Returns nil when no annotations remain.
func filterToolingAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}
	out := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if isToolingAnnotation(k) {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isToolingAnnotation reports whether key is written by a tool rather than
// declared by the user. These are omitted from migration snapshots so a
// rollback does not restore a provenance stamp that no longer applies.
func isToolingAnnotation(key string) bool {
	return strings.HasPrefix(key, "kubectl.kubernetes.io/") ||
		strings.HasPrefix(key, "inrun.dev/")
}

// isFrom reports whether the snapshot was written by a migration from gvk.
func (s kindSnapshot) isFrom(gvk schema.GroupVersionKind) bool {
	return s.Source.APIVersion == gvk.GroupVersion().String() && s.Source.Kind == gvk.Kind
}

// sourceObject rebuilds the source CR recorded in the snapshot.
func (s kindSnapshot) sourceObject() *unstructured.Unstructured {
	src := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": s.Source.APIVersion,
		"kind":       s.Source.Kind,
	}}
	meta := s.Source.Metadata
	src.SetName(meta.Name)
	if meta.Namespace != "" {
		src.SetNamespace(meta.Namespace)
	}
	if len(meta.Labels) > 0 {
		src.SetLabels(meta.Labels)
	}
	if len(meta.Annotations) > 0 {
		src.SetAnnotations(meta.Annotations)
	}
	if s.Source.Spec != nil {
		src.Object["spec"] = runtime.DeepCopyJSONValue(s.Source.Spec)
	}
	if s.Source.Status != nil {
		src.Object["status"] = runtime.DeepCopyJSONValue(s.Source.Status)
	}
	return src
}
