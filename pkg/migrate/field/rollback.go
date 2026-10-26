package field

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/intentd/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// rollbackCR restores a CR from its migrate-field-snapshot annotation. For each
// field in the snapshot it first checks whether the current spec value is still
// the direct product of the original migration (blueprint transform applied to
// the snapshot value). Fields whose value changed after migration are reported
// as drifted and left untouched.
//
// Returns (restored, drifted, afterSpec, error). All three slices are nil when
// the CR carries no snapshot (never migrated or already rolled back).
func (m *migrator) rollbackCR(
	ctx context.Context,
	cr *unstructured.Unstructured,
	resource dynamic.NamespaceableResourceInterface,
	fields []migratableField,
) (restored []string, drifted []string, afterSpec map[string]interface{}, err error) {
	snapshotJSON := cr.GetAnnotations()[labels.AnnotationMigrateFieldSnapshot]
	if snapshotJSON == "" {
		return nil, nil, nil, nil
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return nil, nil, nil, fmt.Errorf("decode snapshot: %w", err)
	}

	spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}

	// Start with every snapshot entry retained; delete only what is successfully
	// restored. Entries for fields no longer in the Blueprint (removed declarations,
	// renamed fields) are preserved automatically — Blueprint changes do not destroy
	// recovery records for fields this rollback does not touch.
	remainingSnapshot := make(map[string]interface{}, len(snapshot))
	for k, v := range snapshot {
		remainingSnapshot[k] = v
	}

	var restoredFrags []map[string]interface{}
	for _, f := range fields {
		raw, ok := snapshot[f.MigrateFrom]
		if !ok {
			continue
		}
		beforeFrag, afterFrag, ok := parseSnapshotEntry(raw)
		if !ok {
			continue
		}
		if fragmentDrifted(spec, afterFrag) {
			// Confirmed drifted: leave this entry in remainingSnapshot so the
			// recovery record is preserved for a future rollback attempt.
			drifted = append(drifted, f.Name)
			continue
		}
		for path, val := range beforeFrag {
			setNested(spec, path, val)
		}
		restoredFrags = append(restoredFrags, beforeFrag)
		restored = append(restored, f.Name)
		delete(remainingSnapshot, f.MigrateFrom)
	}

	if len(restored) == 0 {
		return nil, drifted, nil, nil
	}

	patch, patchBuildErr := buildRollbackPatch(cr, restoredFrags, remainingSnapshot)
	if patchBuildErr != nil {
		return nil, drifted, nil, patchBuildErr
	}
	if err := m.Patch(ctx, resource, cr, patch); err != nil {
		return restored, drifted, spec, fmt.Errorf("apply rollback patch: %w", err)
	}

	return restored, drifted, spec, nil
}

// parseSnapshotEntry decodes a snapshot map entry into its before and after
// flat maps. Both must be {"before": map, "after": map} — any other shape
// returns ok=false and the field is skipped during rollback.
func parseSnapshotEntry(raw interface{}) (before, after map[string]interface{}, ok bool) {
	m, isMap := raw.(map[string]interface{})
	if !isMap {
		return nil, nil, false
	}
	beforeRaw, hasBefore := m["before"]
	afterRaw, hasAfter := m["after"]
	if !hasBefore || !hasAfter {
		return nil, nil, false
	}
	beforeMap, bOk := beforeRaw.(map[string]interface{})
	afterMap, aOk := afterRaw.(map[string]interface{})
	if !bOk || !aOk {
		return nil, nil, false
	}
	return beforeMap, afterMap, true
}

// fragmentDrifted reports whether any non-nil path in afterFrag differs from
// the current spec value at that path. Nil-valued entries represent paths that
// were cleared by migration; their absence from the live spec is expected and
// not treated as drift.
func fragmentDrifted(currentSpec map[string]interface{}, afterFrag map[string]interface{}) bool {
	for path, expected := range afterFrag {
		if expected == nil {
			continue
		}
		current, found := getNested(currentSpec, path)
		if !found {
			// Path was written by migration but is now absent — drift.
			return true
		}
		// Round-trip through JSON to normalise numeric types: the Kubernetes API
		// returns integers as int64, but encoding/json decodes JSON numbers as
		// float64. Without normalisation, int64(5) != float64(5) under
		// reflect.DeepEqual and every integer field would falsely appear drifted.
		if !reflect.DeepEqual(jsonNormalize(current), jsonNormalize(expected)) {
			return true
		}
	}
	return false
}
