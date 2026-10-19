package migrate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// buildMigrationPatch wraps specOf with the CR metadata envelope for a merge-patch call.
// snapshotJSON is the pre-serialized snapshot — callers must marshal it before
// this call so that a serialization failure aborts migration rather than
// producing a patch without the recovery annotation.
func buildMigrationPatch(cr *unstructured.Unstructured, spec map[string]interface{}, fields []migratableField, migrated []string, snapshotJSON []byte, ts time.Time) map[string]interface{} {
	meta := map[string]interface{}{
		"name": cr.GetName(),
		"annotations": map[string]interface{}{
			labels.AnnotationMigrateSnapshot:  string(snapshotJSON),
			labels.AnnotationMigrateTimestamp: ts.UTC().Format(time.RFC3339),
		},
	}
	if ns := cr.GetNamespace(); ns != "" {
		meta["namespace"] = ns
	}
	return map[string]interface{}{
		"apiVersion": cr.GetAPIVersion(),
		"kind":       cr.GetKind(),
		"metadata":   meta,
		"spec":       specOf(spec, fields, migrated),
	}
}

// buildSnapshotEntry constructs the {"before": flatMap, "after": flatMap} entry
// for field f after the transform has run. flatMap keys are dot-separated spec
// paths; values are the corresponding spec values (nil means the path should be
// absent after rollback applies the fragment).
//
// Relocation (MigrateFrom != SpecPath):
//
//	before: {MigrateFrom: oldVal, SpecPath: nil}
//	after:  {SpecPath: newVal, MigrateFrom: nil}
//
// Same-path:
//
//	before: {MigrateFrom: oldVal}
//	after:  {SpecPath: newVal}
//
// values: fanout (one source → many destinations):
//
//	before: {MigrateFrom: oldVal, key1: nil, key2: nil, ...}
//	after:  {key1: v1, key2: v2, ..., MigrateFrom: nil}
func buildSnapshotEntry(spec map[string]interface{}, f migratableField, oldVal interface{}) map[string]interface{} {
	before := map[string]interface{}{f.MigrateFrom: oldVal}
	after := map[string]interface{}{}

	if f.Config.HasValues() {
		for key := range f.Config.Values {
			val, _ := getNested(spec, key)
			after[key] = val
			before[key] = nil
		}
		if _, alreadyInAfter := after[f.MigrateFrom]; !alreadyInAfter {
			after[f.MigrateFrom] = nil
		}
	} else {
		outVal, _ := getNested(spec, f.SpecPath)
		after[f.SpecPath] = outVal
		if f.MigrateFrom != f.SpecPath {
			after[f.MigrateFrom] = nil
			before[f.SpecPath] = nil
		}
	}

	return map[string]interface{}{"before": before, "after": after}
}

// buildRollbackPatch constructs a sparse merge-patch from the before fragments
// of all restored fields and updates the migration annotations.
//
// remainingSnapshot holds all snapshot entries not yet resolved by this
// rollback. When empty both migration annotations are removed (full rollback
// complete). When non-empty the snapshot annotation is updated to those
// entries — fields that drifted, were skipped, or whose declarations were
// removed from the Blueprint retain their recovery records.
func buildRollbackPatch(cr *unstructured.Unstructured, restoredFrags []map[string]interface{}, remainingSnapshot map[string]interface{}) (map[string]interface{}, error) {
	ann := map[string]interface{}{}
	if len(remainingSnapshot) == 0 {
		ann[labels.AnnotationMigrateSnapshot] = nil
		ann[labels.AnnotationMigrateTimestamp] = nil
	} else {
		b, err := json.Marshal(remainingSnapshot)
		if err != nil {
			return nil, fmt.Errorf("marshal remaining snapshot: %w", err)
		}
		ann[labels.AnnotationMigrateSnapshot] = string(b)
	}
	meta := map[string]interface{}{
		"name":        cr.GetName(),
		"annotations": ann,
	}
	if ns := cr.GetNamespace(); ns != "" {
		meta["namespace"] = ns
	}
	patchSpec := map[string]interface{}{}
	for _, frag := range restoredFrags {
		for path, val := range frag {
			setNested(patchSpec, path, val)
		}
	}
	return map[string]interface{}{
		"apiVersion": cr.GetAPIVersion(),
		"kind":       cr.GetKind(),
		"metadata":   meta,
		"spec":       patchSpec,
	}, nil
}

// buildCleanPatch constructs the merge patch that removes both migration
// annotations without touching the spec.
func buildCleanPatch(cr *unstructured.Unstructured) map[string]interface{} {
	meta := map[string]interface{}{
		"name": cr.GetName(),
		"annotations": map[string]interface{}{
			labels.AnnotationMigrateSnapshot:  nil,
			labels.AnnotationMigrateTimestamp: nil,
		},
	}
	if ns := cr.GetNamespace(); ns != "" {
		meta["namespace"] = ns
	}
	return map[string]interface{}{
		"apiVersion": cr.GetAPIVersion(),
		"kind":       cr.GetKind(),
		"metadata":   meta,
	}
}
