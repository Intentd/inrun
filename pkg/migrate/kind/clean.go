package kind

import (
	"context"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// cleanCRKind removes the kind snapshot from a destination. The shared
// timestamp is removed only when no field-level snapshot still needs it.
func (m *migrator) clean(
	ctx context.Context,
	dst *unstructured.Unstructured,
	dstResource dynamic.NamespaceableResourceInterface,
) ([]string, error) {
	ann := dst.GetAnnotations()
	if _, ok := ann[labels.AnnotationMigrateKindSnapshot]; !ok {
		return nil, nil
	}

	remove := map[string]interface{}{labels.AnnotationMigrateKindSnapshot: nil}
	if _, hasField := ann[labels.AnnotationMigrateFieldSnapshot]; !hasField {
		remove[labels.AnnotationMigrateTimestamp] = nil
	}
	meta := map[string]interface{}{"name": dst.GetName(), "annotations": remove}
	if ns := dst.GetNamespace(); ns != "" {
		meta["namespace"] = ns
	}
	if err := m.Patch(ctx, dstResource, dst, map[string]interface{}{"metadata": meta}); err != nil {
		return nil, err
	}
	return []string{"annotations"}, nil
}

func (m *migrator) processClean(
	ctx context.Context,
	dst *unstructured.Unstructured,
	dstResource dynamic.NamespaceableResourceInterface,
	log zerolog.Logger,
) (core.CRResult, string) {
	changed, err := m.clean(ctx, dst, dstResource)
	if err != nil {
		return m.Fail(dst, err, log)
	}
	return core.CRResult{
		Namespace: dst.GetNamespace(),
		Name:      dst.GetName(),
		Fields:    changed,
		Skipped:   len(changed) == 0,
	}, ""
}
