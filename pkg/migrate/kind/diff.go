package kind

import (
	"encoding/json"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// snapshotSource returns the source recorded on a destination's snapshot.
func snapshotSource(dst *unstructured.Unstructured) (kindSnapshotSource, bool) {
	raw := dst.GetAnnotations()[labels.AnnotationMigrateKindSnapshot]
	if raw == "" {
		return kindSnapshotSource{}, false
	}
	var snap kindSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return kindSnapshotSource{}, false
	}
	return snap.Source, true
}

// is reports whether s records cr.
func (s kindSnapshotSource) is(cr *unstructured.Unstructured) bool {
	return s.APIVersion == cr.GetAPIVersion() && s.Kind == cr.GetKind() &&
		s.Metadata.Namespace == cr.GetNamespace() && s.Metadata.Name == cr.GetName()
}

func (s kindSnapshotSource) ref() string {
	return s.Kind + " " + core.MetaRef(s.Metadata.Namespace, s.Metadata.Name)
}
