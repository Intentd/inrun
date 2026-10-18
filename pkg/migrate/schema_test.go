package migrate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeCRDWithMigrate(from string) *types.CRDEntry {
	return &types.CRDEntry{
		Serve: &types.ServeConfig{
			Fields: map[string]types.ServeFieldConfig{
				"schedule": {
					Migrate: &types.ServeFieldMigrate{From: from},
				},
			},
		},
	}
}

func TestMigratableFields_SpecPrefixStripped(t *testing.T) {
	fields := migratableFields(makeCRDWithMigrate("spec.schedule"))
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].MigrateFrom != "schedule" {
		t.Errorf("MigrateFrom = %q, want %q", fields[0].MigrateFrom, "schedule")
	}
}

func TestMigratableFields_BarePathUnchanged(t *testing.T) {
	fields := migratableFields(makeCRDWithMigrate("schedule"))
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].MigrateFrom != "schedule" {
		t.Errorf("MigrateFrom = %q, want %q", fields[0].MigrateFrom, "schedule")
	}
}

func TestMigratableFields_NestedSpecPrefixStripped(t *testing.T) {
	fields := migratableFields(makeCRDWithMigrate("spec.foo.bar"))
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].MigrateFrom != "foo.bar" {
		t.Errorf("MigrateFrom = %q, want %q", fields[0].MigrateFrom, "foo.bar")
	}
}

func TestMigratableFields_NoMigrate(t *testing.T) {
	crd := &types.CRDEntry{
		Serve: &types.ServeConfig{
			Fields: map[string]types.ServeFieldConfig{
				"image": {},
			},
		},
	}
	if got := migratableFields(crd); len(got) != 0 {
		t.Errorf("expected 0 fields, got %d", len(got))
	}
}

func TestMigratableFields_NilServe(t *testing.T) {
	if got := migratableFields(&types.CRDEntry{}); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func fieldOf(name, from string) migratableField {
	return migratableField{Name: name, SpecPath: name, MigrateFrom: from, Config: types.ServeFieldConfig{}}
}

// fieldWithPath creates a migratableField where the write destination differs
// from the name — the relocation case (e.g. replicas → scaling.min).
func fieldWithPath(name, specPath, from string) migratableField {
	return migratableField{
		Name:        name,
		SpecPath:    specPath,
		MigrateFrom: from,
		Config:      types.ServeFieldConfig{Path: specPath},
	}
}

func TestPrepareField_ScalarReturnsValueAndDeletes(t *testing.T) {
	spec := map[string]interface{}{"schedule": "0 2 * * 1-5", "image": "busybox"}
	val, ok := prepareField(spec, fieldOf("schedule", "schedule"))
	if !ok {
		t.Fatal("expected ok=true for scalar value")
	}
	if val != "0 2 * * 1-5" {
		t.Errorf("val = %v, want %q", val, "0 2 * * 1-5")
	}
	if _, found := structured.GetNestedPath(spec, "schedule"); found {
		t.Error("expected migrate.from to be deleted from spec")
	}
}

func TestPrepareField_AbsentSkips(t *testing.T) {
	spec := map[string]interface{}{"image": "busybox"}
	_, ok := prepareField(spec, fieldOf("schedule", "schedule"))
	if ok {
		t.Error("expected ok=false when migrate.from is absent")
	}
}

func TestPrepareField_SameKeyAlreadyMapSkips(t *testing.T) {
	// migrate.from == field.Name; value is already a structured map — migrated.
	spec := map[string]interface{}{
		"schedule": map[string]interface{}{"minute": "0", "hour": "2"},
	}
	_, ok := prepareField(spec, fieldOf("schedule", "schedule"))
	if ok {
		t.Error("expected ok=false when same-key value is already a map")
	}
}

func TestPrepareField_DifferentKeyMapDoesNotSkip(t *testing.T) {
	// migrate.from != field.Name; the old key holds a map for unrelated reasons —
	// prepareField must not skip, only same-key map detection applies.
	spec := map[string]interface{}{
		"oldConfig": map[string]interface{}{"key": "val"},
	}
	val, ok := prepareField(spec, fieldOf("newConfig", "oldConfig"))
	if !ok {
		t.Error("expected ok=true: different-key map should not be skipped")
	}
	if val == nil {
		t.Error("expected non-nil val")
	}
}

func TestPrepareField_RelocationScalarMigratesEveryTime(t *testing.T) {
	// replicas → scaling.min: from == name != specPath.
	// The old scalar is never replaced in-place, so the map-skip must NOT fire.
	spec := map[string]interface{}{"replicas": 3}
	f := fieldWithPath("replicas", "scaling.min", "replicas")
	val, ok := prepareField(spec, f)
	if !ok {
		t.Fatal("expected ok=true: relocation scalar should not be skipped")
	}
	if val != 3 {
		t.Errorf("val = %v, want 3", val)
	}
	if _, found := structured.GetNestedPath(spec, "replicas"); found {
		t.Error("expected migrate.from deleted after prepareField")
	}
}

func TestPrepareField_RelocationAfterMigrationSkips(t *testing.T) {
	// After a successful relocation migration the old key is absent (null-patched).
	// Re-run should skip because the old path is gone.
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": 3},
		// "replicas" is absent — server removed it after the first migration.
	}
	f := fieldWithPath("replicas", "scaling.min", "replicas")
	_, ok := prepareField(spec, f)
	if ok {
		t.Error("expected ok=false: old path absent means already migrated")
	}
}

func TestSparsePatch_OnlyMigratedPathsPresent(t *testing.T) {
	// spec has three fields; only replicas and logLevel are migrated. image must
	// be absent from the patch so a concurrent update to it is not overwritten.
	spec := map[string]interface{}{
		"scaling":  map[string]interface{}{"min": 3},
		"logLevel": "info",
		"image":    "myapp:latest",
	}
	fields := []migratableField{
		fieldWithPath("replicas", "scaling.min", "replicas"),
		fieldOf("logLevel", "logLevel"),
	}

	patch := specOf(spec, fields, []string{"replicas", "logLevel"})

	if _, ok := patch["image"]; ok {
		t.Error("image must not appear in sparse patch")
	}
	scalingRaw, ok := patch["scaling"]
	if !ok {
		t.Fatal("scaling must appear in sparse patch")
	}
	scaling, ok := scalingRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("scaling must be a map, got %T", scalingRaw)
	}
	if scaling["min"] != 3 {
		t.Errorf("scaling.min = %v, want 3", scaling["min"])
	}
	if patch["logLevel"] != "info" {
		t.Errorf("logLevel = %v, want info", patch["logLevel"])
	}
}

func TestSparsePatch_RelocationNullsOldKey(t *testing.T) {
	// replicas relocated to scaling.min; old key must be explicitly nulled so the
	// server removes it (merge patch only removes keys set to null).
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": 5},
	}
	fields := []migratableField{
		fieldWithPath("replicas", "scaling.min", "replicas"),
	}

	patch := specOf(spec, fields, []string{"replicas"})

	nullVal, hasOld := patch["replicas"]
	if !hasOld {
		t.Fatal("old key 'replicas' must be present in patch (set to null)")
	}
	if nullVal != nil {
		t.Errorf("old key 'replicas' must be null, got %v", nullVal)
	}
}

func TestSparsePatch_SamePathNoNullEntry(t *testing.T) {
	// logLevel migrates in-place (from == specPath); patch must not add a null
	// entry under the same key.
	spec := map[string]interface{}{
		"logLevel": "debug",
	}
	fields := []migratableField{
		fieldOf("logLevel", "logLevel"),
	}

	patch := specOf(spec, fields, []string{"logLevel"})

	val, ok := patch["logLevel"]
	if !ok {
		t.Fatal("logLevel must appear in patch")
	}
	if val == nil {
		t.Error("logLevel must not be null for same-path migration")
	}
}

// ── buildSnapshotEntry tests ────────────────────────────────────────────────

func TestBuildSnapshotEntry_Relocation(t *testing.T) {
	// replicas → scaling.min: before has {replicas: 5, scaling.min: nil},
	// after has {scaling.min: 5, replicas: nil}.
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": 5},
		// "replicas" is absent — prepareField deletes it before this is called.
	}
	f := fieldWithPath("replicas", "scaling.min", "replicas")
	entry := buildSnapshotEntry(spec, f, 5)

	before, bOk := entry["before"].(map[string]interface{})
	after, aOk := entry["after"].(map[string]interface{})
	if !bOk || !aOk {
		t.Fatal("before and after must be maps")
	}

	if before["replicas"] != 5 {
		t.Errorf("before[replicas] = %v, want 5", before["replicas"])
	}
	if v, ok := before["scaling.min"]; !ok || v != nil {
		t.Errorf("before[scaling.min] = %v, want nil", v)
	}
	if after["scaling.min"] != 5 {
		t.Errorf("after[scaling.min] = %v, want 5", after["scaling.min"])
	}
	if v, ok := after["replicas"]; !ok || v != nil {
		t.Errorf("after[replicas] = %v, want nil", v)
	}
}

func TestBuildSnapshotEntry_SamePath(t *testing.T) {
	// logLevel in-place (MigrateFrom == SpecPath): no null entries, one key each.
	spec := map[string]interface{}{"logLevel": "info"}
	f := fieldOf("logLevel", "logLevel")
	entry := buildSnapshotEntry(spec, f, "info")

	before, bOk := entry["before"].(map[string]interface{})
	after, aOk := entry["after"].(map[string]interface{})
	if !bOk || !aOk {
		t.Fatal("before and after must be maps")
	}
	if before["logLevel"] != "info" {
		t.Errorf("before[logLevel] = %v, want info", before["logLevel"])
	}
	if after["logLevel"] != "info" {
		t.Errorf("after[logLevel] = %v, want info", after["logLevel"])
	}
	if len(before) != 1 || len(after) != 1 {
		t.Errorf("same-path entry should have exactly 1 key each; before=%v after=%v", before, after)
	}
}

// ── fragmentDrifted tests ───────────────────────────────────────────────────

func TestFragmentDrifted_NotDrifted(t *testing.T) {
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": float64(5)},
	}
	afterFrag := map[string]interface{}{"scaling.min": float64(5)}
	if fragmentDrifted(spec, afterFrag) {
		t.Error("expected not drifted when values match")
	}
}

func TestFragmentDrifted_Drifted(t *testing.T) {
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": float64(99)},
	}
	afterFrag := map[string]interface{}{"scaling.min": float64(5)}
	if !fragmentDrifted(spec, afterFrag) {
		t.Error("expected drifted when current differs from after")
	}
}

func TestFragmentDrifted_NumericNormalisation(t *testing.T) {
	// Kubernetes API returns int64; snapshot JSON decodes to float64.
	spec := map[string]interface{}{
		"scaling": map[string]interface{}{"min": int64(5)},
	}
	afterFrag := map[string]interface{}{"scaling.min": float64(5)}
	if fragmentDrifted(spec, afterFrag) {
		t.Error("int64(5) and float64(5) must not be treated as drifted")
	}
}

func TestFragmentDrifted_NilEntrySkipped(t *testing.T) {
	// Nil entries in afterFrag represent cleared source paths — absence is fine.
	spec := map[string]interface{}{}
	afterFrag := map[string]interface{}{"replicas": nil}
	if fragmentDrifted(spec, afterFrag) {
		t.Error("nil entry in after frag should be skipped, not counted as drift")
	}
}

func TestFragmentDrifted_AbsentWhenExpectedValue(t *testing.T) {
	// A path that migration wrote a real value to, then someone deleted it.
	spec := map[string]interface{}{}
	afterFrag := map[string]interface{}{"scaling.min": float64(5)}
	if !fragmentDrifted(spec, afterFrag) {
		t.Error("expected drifted when migration wrote a value but it is now absent")
	}
}

// ── parseSnapshotEntry tests ────────────────────────────────────────────────

func TestParseSnapshotEntry_ValidEntry(t *testing.T) {
	raw := map[string]interface{}{
		"before": map[string]interface{}{"replicas": float64(5), "scaling.min": nil},
		"after":  map[string]interface{}{"scaling.min": float64(5), "replicas": nil},
	}
	before, after, ok := parseSnapshotEntry(raw)
	if !ok {
		t.Fatal("expected ok=true for valid entry")
	}
	if before["replicas"] != float64(5) {
		t.Errorf("before[replicas] = %v, want 5", before["replicas"])
	}
	if after["scaling.min"] != float64(5) {
		t.Errorf("after[scaling.min] = %v, want 5", after["scaling.min"])
	}
}

func TestParseSnapshotEntry_NotAMap(t *testing.T) {
	_, _, ok := parseSnapshotEntry("raw-scalar")
	if ok {
		t.Error("expected ok=false for non-map entry")
	}
}

func TestParseSnapshotEntry_MissingBefore(t *testing.T) {
	raw := map[string]interface{}{
		"after": map[string]interface{}{"scaling.min": float64(5)},
	}
	_, _, ok := parseSnapshotEntry(raw)
	if ok {
		t.Error("expected ok=false when 'before' key is missing")
	}
}

func TestParseSnapshotEntry_BeforeNotMap(t *testing.T) {
	raw := map[string]interface{}{
		"before": "scalar",
		"after":  map[string]interface{}{"scaling.min": float64(5)},
	}
	_, _, ok := parseSnapshotEntry(raw)
	if ok {
		t.Error("expected ok=false when 'before' is not a map")
	}
}

func TestParseSnapshotEntry_MissingAfter(t *testing.T) {
	raw := map[string]interface{}{
		"before": map[string]interface{}{"replicas": float64(5)},
	}
	_, _, ok := parseSnapshotEntry(raw)
	if ok {
		t.Error("expected ok=false when 'after' key is missing")
	}
}

func TestParseSnapshotEntry_AfterNotMap(t *testing.T) {
	raw := map[string]interface{}{
		"before": map[string]interface{}{"replicas": float64(5)},
		"after":  "scalar",
	}
	_, _, ok := parseSnapshotEntry(raw)
	if ok {
		t.Error("expected ok=false when 'after' is not a map")
	}
}

// ── buildSnapshotEntry values: fanout test ──────────────────────────────────

func TestBuildSnapshotEntry_ValuesFanout(t *testing.T) {
	// "tier" → {cpu: "500m", memory: "512Mi"}: one source key, two destinations.
	spec := map[string]interface{}{
		"resources": map[string]interface{}{
			"cpu":    "500m",
			"memory": "512Mi",
		},
	}
	f := migratableField{
		Name:        "tier",
		SpecPath:    "tier",
		MigrateFrom: "tier",
		Config: types.ServeFieldConfig{
			Values: map[string]string{
				"resources.cpu":    "500m",
				"resources.memory": "512Mi",
			},
		},
	}
	entry := buildSnapshotEntry(spec, f, "standard")

	before, bOk := entry["before"].(map[string]interface{})
	after, aOk := entry["after"].(map[string]interface{})
	if !bOk || !aOk {
		t.Fatal("before and after must be maps")
	}

	// Source key recorded with old value in before.
	if before["tier"] != "standard" {
		t.Errorf("before[tier] = %v, want standard", before["tier"])
	}
	// Destination keys are nil in before (they didn't exist yet).
	for _, key := range []string{"resources.cpu", "resources.memory"} {
		v, ok := before[key]
		if !ok {
			t.Errorf("before[%s] missing", key)
		} else if v != nil {
			t.Errorf("before[%s] = %v, want nil", key, v)
		}
	}
	// Destination keys hold the written values in after.
	if after["resources.cpu"] != "500m" {
		t.Errorf("after[resources.cpu] = %v, want 500m", after["resources.cpu"])
	}
	if after["resources.memory"] != "512Mi" {
		t.Errorf("after[resources.memory] = %v, want 512Mi", after["resources.memory"])
	}
	// Source key is nil in after (removed).
	v, ok := after["tier"]
	if !ok {
		t.Error("after[tier] missing")
	} else if v != nil {
		t.Errorf("after[tier] = %v, want nil", v)
	}
}

// ── buildRollbackPatch tests ────────────────────────────────────────────────

func makeUnstructured(apiVersion, kind, namespace, name string, annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	if annotations != nil {
		u.SetAnnotations(annotations)
	}
	return u
}

func TestBuildRollbackPatch_FullRollback(t *testing.T) {
	// All fields restored; remainingSnapshot is empty → both annotations nil'd.
	cr := makeUnstructured("example.io/v1", "WebApp", "default", "my-app", map[string]string{
		labels.AnnotationMigrateSnapshot:  `{"schedule":{"before":{"schedule":"0 2 * * 1-5"},"after":{"schedule":"0 2 * * 1-5"}}}`,
		labels.AnnotationMigrateTimestamp: "2026-10-15T00:00:00Z",
	})

	restoredFrags := []map[string]interface{}{
		{"schedule": "0 2 * * 1-5"},
	}

	patch, err := buildRollbackPatch(cr, restoredFrags, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta, _ := patch["metadata"].(map[string]interface{})
	ann, _ := meta["annotations"].(map[string]interface{})

	if v, ok := ann[labels.AnnotationMigrateSnapshot]; !ok || v != nil {
		t.Errorf("snapshot annotation must be nil after full rollback, got %v", v)
	}
	if v, ok := ann[labels.AnnotationMigrateTimestamp]; !ok || v != nil {
		t.Errorf("timestamp annotation must be nil after full rollback, got %v", v)
	}

	spec, _ := patch["spec"].(map[string]interface{})
	if spec["schedule"] != "0 2 * * 1-5" {
		t.Errorf("spec.schedule = %v, want 0 2 * * 1-5", spec["schedule"])
	}
}

func TestBuildRollbackPatch_PartialRollback(t *testing.T) {
	// One field drifted; remainingSnapshot is non-empty → snapshot annotation updated.
	remaining := map[string]interface{}{
		"image": map[string]interface{}{
			"before": map[string]interface{}{"image": "myapp:1"},
			"after":  map[string]interface{}{"image": "myapp:2"},
		},
	}
	cr := makeUnstructured("example.io/v1", "WebApp", "default", "my-app", nil)

	restoredFrags := []map[string]interface{}{
		{"schedule": "0 2 * * 1-5"},
	}

	patch, err := buildRollbackPatch(cr, restoredFrags, remaining)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta, _ := patch["metadata"].(map[string]interface{})
	ann, _ := meta["annotations"].(map[string]interface{})

	snapshotVal, ok := ann[labels.AnnotationMigrateSnapshot]
	if !ok {
		t.Fatal("snapshot annotation must be present in partial rollback")
	}
	snapshotStr, isStr := snapshotVal.(string)
	if !isStr || snapshotStr == "" {
		t.Errorf("snapshot annotation must be a non-empty string, got %T %v", snapshotVal, snapshotVal)
	}

	// Timestamp is not touched in partial rollback.
	if _, hasTS := ann[labels.AnnotationMigrateTimestamp]; hasTS {
		t.Error("timestamp annotation must not appear in partial rollback patch")
	}
}

func TestBuildRollbackPatch_NamespacedCR(t *testing.T) {
	// Namespace must appear in the patch metadata.
	cr := makeUnstructured("example.io/v1", "WebApp", "team-ns", "svc", nil)

	patch, err := buildRollbackPatch(cr, []map[string]interface{}{{"replicas": float64(1)}}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta, _ := patch["metadata"].(map[string]interface{})
	if meta["namespace"] != "team-ns" {
		t.Errorf("namespace = %v, want team-ns", meta["namespace"])
	}
}

func TestBuildRollbackPatch_RelocatedField(t *testing.T) {
	// replicas → scaling.min: before fragment has a flat "scaling.min" path that
	// must be nested into spec correctly.
	cr := makeUnstructured("example.io/v1", "WebApp", "", "svc", nil)

	restoredFrags := []map[string]interface{}{
		{"scaling.min": float64(3), "replicas": nil},
	}

	patch, err := buildRollbackPatch(cr, restoredFrags, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec, _ := patch["spec"].(map[string]interface{})
	scalingRaw, ok := spec["scaling"]
	if !ok {
		t.Fatal("spec.scaling must be present")
	}
	scaling, ok := scalingRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("spec.scaling must be a map, got %T", scalingRaw)
	}
	if scaling["min"] != float64(3) {
		t.Errorf("scaling.min = %v, want 3", scaling["min"])
	}
	if _, hasReplicas := spec["replicas"]; !hasReplicas {
		t.Error("spec.replicas must be present (set to nil to remove via merge patch)")
	}
}

// ── buildMigrationPatch tests ───────────────────────────────────────────────

func TestBuildMigrationPatch_SnapshotAndTimestampSet(t *testing.T) {
	// After a migration the patch must carry both migration annotations.
	cr := makeUnstructured("example.io/v1", "WebApp", "default", "my-app", nil)
	spec := map[string]interface{}{"schedule": "0 2 * * 1-5"}
	fields := []migratableField{fieldOf("schedule", "schedule")}
	snapshot, _ := json.Marshal(map[string]interface{}{
		"schedule": map[string]interface{}{
			"before": map[string]interface{}{"schedule": "*/5 * * * *"},
			"after":  map[string]interface{}{"schedule": "0 2 * * 1-5"},
		},
	})
	ts := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	patch := buildMigrationPatch(cr, spec, fields, []string{"schedule"}, snapshot, ts)

	meta, _ := patch["metadata"].(map[string]interface{})
	ann, _ := meta["annotations"].(map[string]interface{})

	if ann[labels.AnnotationMigrateSnapshot] != string(snapshot) {
		t.Errorf("snapshot annotation = %v, want %s", ann[labels.AnnotationMigrateSnapshot], snapshot)
	}
	if ann[labels.AnnotationMigrateTimestamp] != "2026-10-15T12:00:00Z" {
		t.Errorf("timestamp annotation = %v, want 2026-10-15T12:00:00Z", ann[labels.AnnotationMigrateTimestamp])
	}
}

func TestBuildMigrationPatch_SpecContainsMigratedFieldsOnly(t *testing.T) {
	// spec has 3 fields; only "schedule" was migrated — "image" and "replicas" must
	// be absent from the sparse patch so concurrent writes are not overwritten.
	cr := makeUnstructured("example.io/v1", "WebApp", "", "svc", nil)
	spec := map[string]interface{}{
		"schedule": "0 2 * * 1-5",
		"image":    "myapp:latest",
		"replicas": float64(3),
	}
	fields := []migratableField{
		fieldOf("schedule", "schedule"),
		fieldOf("image", "image"),
		fieldOf("replicas", "replicas"),
	}

	patch := buildMigrationPatch(cr, spec, fields, []string{"schedule"}, []byte(`{}`), time.Now())

	patchSpec, _ := patch["spec"].(map[string]interface{})
	if _, ok := patchSpec["image"]; ok {
		t.Error("image must not appear in sparse patch")
	}
	if _, ok := patchSpec["replicas"]; ok {
		t.Error("replicas must not appear in sparse patch")
	}
	if patchSpec["schedule"] != "0 2 * * 1-5" {
		t.Errorf("schedule = %v, want 0 2 * * 1-5", patchSpec["schedule"])
	}
}

// ── buildCleanPatch tests ───────────────────────────────────────────────────

func TestBuildCleanPatch_BothAnnotationsNilld(t *testing.T) {
	// Patch must nil both migration annotations and carry no spec.
	cr := makeUnstructured("example.io/v1", "WebApp", "default", "my-app", map[string]string{
		labels.AnnotationMigrateSnapshot:  `{"schedule":{}}`,
		labels.AnnotationMigrateTimestamp: "2026-10-15T00:00:00Z",
	})

	patch := buildCleanPatch(cr)

	meta, _ := patch["metadata"].(map[string]interface{})
	ann, _ := meta["annotations"].(map[string]interface{})

	if v, ok := ann[labels.AnnotationMigrateSnapshot]; !ok || v != nil {
		t.Errorf("snapshot annotation must be nil, got %v", v)
	}
	if v, ok := ann[labels.AnnotationMigrateTimestamp]; !ok || v != nil {
		t.Errorf("timestamp annotation must be nil, got %v", v)
	}
	if _, hasSpec := patch["spec"]; hasSpec {
		t.Error("clean patch must not include a spec key")
	}
}

func TestBuildCleanPatch_NamespacedCR(t *testing.T) {
	cr := makeUnstructured("example.io/v1", "WebApp", "team-ns", "svc", nil)
	patch := buildCleanPatch(cr)
	meta, _ := patch["metadata"].(map[string]interface{})
	if meta["namespace"] != "team-ns" {
		t.Errorf("namespace = %v, want team-ns", meta["namespace"])
	}
}

func TestBuildCleanPatch_ClusterScopedCR(t *testing.T) {
	// Cluster-scoped CRs have no namespace — must not appear in patch metadata.
	cr := makeUnstructured("example.io/v1", "ClusterApp", "", "global-svc", nil)
	patch := buildCleanPatch(cr)
	meta, _ := patch["metadata"].(map[string]interface{})
	if _, hasNS := meta["namespace"]; hasNS {
		t.Error("namespace must not appear in patch metadata for cluster-scoped CRs")
	}
}

// ── prepareField abort test ─────────────────────────────────────────────────

// TestPrepareField_AbortedAfterDelete documents that an abort between
// prepareField (which deletes) and ApplyFieldTransform leaves migrate.from
// absent. Re-run skips cleanly rather than erroring.
func TestPrepareField_AbortedAfterDelete(t *testing.T) {
	spec := map[string]interface{}{"schedule": "0 2 * * 1-5", "image": "busybox"}
	f := fieldOf("schedule", "schedule")

	prepareField(spec, f) // simulates the delete half of a crashed migration

	if _, found := structured.GetNestedPath(spec, "schedule"); found {
		t.Fatal("expected migrate.from absent after prepare")
	}
	_, ok := prepareField(spec, f) // re-run
	if ok {
		t.Error("expected re-run to skip when migrate.from is absent")
	}
}
