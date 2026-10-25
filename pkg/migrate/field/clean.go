package field

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// cleanCR removes the migration annotations from cr without touching the spec.
// Returns a non-nil fields slice when annotations were present (or dry-run would
// remove them); nil when the CR carries no migration annotations.
func (m *migrator) cleanCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
) ([]string, error) {
	ann := cr.GetAnnotations()
	_, hasSnapshot := ann[labels.AnnotationMigrateFieldSnapshot]
	_, hasKind := ann[labels.AnnotationMigrateKindSnapshot]
	_, hasTimestamp := ann[labels.AnnotationMigrateTimestamp]
	// A timestamp alone is field-level leftover unless a kind snapshot owns it.
	if !hasSnapshot && (!hasTimestamp || hasKind) {
		return nil, nil
	}

	patch := buildCleanPatch(cr)
	if err := m.Patch(ctx, resource, cr, patch); err != nil {
		return []string{"annotations"}, fmt.Errorf("apply clean patch: %w", err)
	}

	return []string{"annotations"}, nil
}
