package kind

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var nop = zerolog.Nop()

func spec() map[string]interface{} {
	return map[string]interface{}{"image": "app:1", "replicas": int64(2)}
}

func forward(t *testing.T, f *fakeKube, cr *unstructured.Unstructured, prune, dryRun bool) core.CRResult {
	t.Helper()
	res, strict := f.migrator(core.ModeForward, dryRun).processForward(
		context.Background(), cr, kindCRD(prune), f.dyn.Resource(dstGVR), false, nop)
	assert.Empty(t, strict)
	return res
}

// rollbackAll lists migrated destinations and rolls each back.
func rollbackAll(t *testing.T, f *fakeKube, dryRun bool) []core.CRResult {
	t.Helper()
	m := f.migrator(core.ModeRollback, dryRun)
	items, err := m.listMigrated(context.Background(), core.Selection{}, srcGVK, f.dyn.Resource(dstGVR), false)
	require.NoError(t, err)
	var out []core.CRResult
	for i := range items {
		res, _ := m.processRollback(context.Background(), &items[i], nop)
		out = append(out, res)
	}
	return out
}

// ── forward ──────────────────────────────────────────────────────────────────

func TestForward_CreatesDestinationKeepsSource(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)

	res := forward(t, f, src, false, false)
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"spec"}, res.Fields)
	assert.False(t, res.Pruned)

	dst := f.get(dstGVR, "team", "app")
	require.NotNil(t, dst)
	assert.Equal(t, "New", dst.GetKind())
	assert.Equal(t, spec()["image"], dst.Object["spec"].(map[string]interface{})["image"])
	assert.Contains(t, dst.GetAnnotations(), labels.AnnotationMigrateKindSnapshot)
	assert.Equal(t, "demo", dst.GetLabels()["app"])
	assert.NotNil(t, f.get(srcGVR, "team", "app"), "source kept without prune")
}

func TestForward_PruneDeletesSource(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)

	res := forward(t, f, src, true, false)
	require.NoError(t, res.Err)
	assert.True(t, res.Pruned)
	assert.Nil(t, f.get(srcGVR, "team", "app"))
	assert.NotNil(t, f.get(dstGVR, "team", "app"))
}

func TestForward_SecondRunSkips(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	res := forward(t, f, src, false, false)
	assert.True(t, res.Skipped)
	assert.NoError(t, res.Err)
}

func TestForward_ServerDefaultsAreNotDrift(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	f.shape = func(u *unstructured.Unstructured) {
		_ = unstructured.SetNestedField(u.Object, "IfNotPresent", "spec", "pullPolicy")
	}
	forward(t, f, src, false, false)

	assert.True(t, forward(t, f, src, false, false).Skipped)
}

func TestForward_AlreadyMigratedFinishesPrune(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false) // earlier run, prune off (or failed)

	res := forward(t, f, src, true, false)
	require.NoError(t, res.Err)
	assert.True(t, res.Pruned)
	assert.Empty(t, res.Fields)
	assert.Nil(t, f.get(srcGVR, "team", "app"))
}

func TestForward_DriftedDestination(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	edited := sourceCR("app", map[string]interface{}{"image": "app:2", "replicas": int64(2)})
	res := forward(t, f, edited, true, false)
	assert.True(t, res.Drifted)
	assert.False(t, res.Pruned)
	assert.NotNil(t, f.get(srcGVR, "team", "app"), "drifted source is not pruned")
}

func TestForward_DestinationFromAnotherSourceIsError(t *testing.T) {
	a := sourceCR("app", spec())
	f := newFakeKube(a)
	forward(t, f, a, false, false)

	// Same name, different namespace: lands on the same destination when the
	// destination is cluster-scoped. Simulate by renaming the namespace only.
	b := sourceCR("app", spec())
	b.SetNamespace("other")
	dstRes := f.dyn.Resource(dstGVR)
	res, strict := f.migrator(core.ModeForward, false).processForward(
		context.Background(), b, kindCRD(false), &sameNamespace{dstRes, "team"}, false, nop)
	assert.Empty(t, strict)
	require.Error(t, res.Err)
	assert.ErrorContains(t, res.Err, "already migrated from Old team/app")
}

func TestForward_SchemaDropsFieldFailsAndWritesNothing(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	f.shape = func(u *unstructured.Unstructured) { unstructured.RemoveNestedField(u.Object, "spec", "replicas") }

	res := forward(t, f, src, true, false)
	require.Error(t, res.Err)
	assert.ErrorContains(t, res.Err, "drops or changes fields: replicas")
	assert.Nil(t, f.get(dstGVR, "team", "app"))
	assert.NotNil(t, f.get(srcGVR, "team", "app"))
}

func TestForward_DryRunWritesNothing(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)

	res := forward(t, f, src, true, true)
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"spec"}, res.Fields)
	assert.True(t, res.Pruned, "dry-run reports the prune it would do")
	assert.Nil(t, f.get(dstGVR, "team", "app"))
	assert.NotNil(t, f.get(srcGVR, "team", "app"))
}

// ── rollback ─────────────────────────────────────────────────────────────────

func TestRollback_RecreatesPrunedSourceThenDeletesDestination(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, true, false)
	require.Nil(t, f.get(srcGVR, "team", "app"))

	res := rollbackAll(t, f, false)
	require.Len(t, res, 1)
	require.NoError(t, res[0].Err)
	assert.Equal(t, []string{"spec"}, res[0].Fields)

	back := f.get(srcGVR, "team", "app")
	require.NotNil(t, back)
	assert.Equal(t, "app:1", back.Object["spec"].(map[string]interface{})["image"])
	assert.Equal(t, "demo", back.GetLabels()["app"])
	assert.Nil(t, f.get(dstGVR, "team", "app"))
}

func TestRollback_NotPrunedOnlyDeletesDestination(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	res := rollbackAll(t, f, false)
	require.Len(t, res, 1)
	require.NoError(t, res[0].Err)
	assert.NotNil(t, f.get(srcGVR, "team", "app"))
	assert.Nil(t, f.get(dstGVR, "team", "app"))
}

func TestRollback_DriftedDestinationLeftInPlace(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, true, false)

	dst := f.get(dstGVR, "team", "app")
	_ = unstructured.SetNestedField(dst.Object, "app:9", "spec", "image")
	_, err := f.dyn.Resource(dstGVR).Namespace("team").Update(context.Background(), dst, metav1UpdateOptions())
	require.NoError(t, err)

	res := rollbackAll(t, f, false)
	require.Len(t, res, 1)
	assert.True(t, res[0].AllDrifted)
	assert.NotNil(t, f.get(dstGVR, "team", "app"))
	assert.Nil(t, f.get(srcGVR, "team", "app"))
}

func TestRollback_DryRunChangesNothing(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, true, false)

	res := rollbackAll(t, f, true)
	require.Len(t, res, 1)
	require.NoError(t, res[0].Err)
	assert.Equal(t, []string{"spec"}, res[0].Fields)
	assert.NotNil(t, f.get(dstGVR, "team", "app"))
	assert.Nil(t, f.get(srcGVR, "team", "app"))
}

func TestRollback_IgnoresDestinationsFromOtherSources(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	m := f.migrator(core.ModeRollback, false)
	other := srcGVK
	other.Kind = "Other"
	items, err := m.listMigrated(context.Background(), core.Selection{}, other, f.dyn.Resource(dstGVR), false)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestRollback_SelectionMatchesRecordedSource(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)
	m := f.migrator(core.ModeRollback, false)

	in, err := m.listMigrated(context.Background(), core.Selection{Selector: "app=demo"}, srcGVK, f.dyn.Resource(dstGVR), false)
	require.NoError(t, err)
	assert.Len(t, in, 1)

	out, err := m.listMigrated(context.Background(), core.Selection{Namespaces: []string{"elsewhere"}}, srcGVK, f.dyn.Resource(dstGVR), false)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// ── clean ────────────────────────────────────────────────────────────────────

func TestClean_RemovesKindSnapshot(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	m := f.migrator(core.ModeClean, false)
	dst := f.get(dstGVR, "team", "app")
	res, _ := m.processClean(context.Background(), dst, f.dyn.Resource(dstGVR), nop)
	require.NoError(t, res.Err)
	assert.Equal(t, []string{"annotations"}, res.Fields)

	ann := f.get(dstGVR, "team", "app").GetAnnotations()
	assert.NotContains(t, ann, labels.AnnotationMigrateKindSnapshot)
	assert.NotContains(t, ann, labels.AnnotationMigrateTimestamp)

	again, _ := m.processClean(context.Background(), f.get(dstGVR, "team", "app"), f.dyn.Resource(dstGVR), nop)
	assert.True(t, again.Skipped)
}

func TestRollback_RecreatesSourcePrunedByLaterRun(t *testing.T) {
	// First run without prune writes "pruned": false; a later run with prune
	// deletes the source. Rollback must still bring it back.
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)
	require.True(t, forward(t, f, src, true, false).Pruned)
	require.Nil(t, f.get(srcGVR, "team", "app"))

	res := rollbackAll(t, f, false)
	require.Len(t, res, 1)
	require.NoError(t, res[0].Err)
	assert.NotNil(t, f.get(srcGVR, "team", "app"))
	assert.Nil(t, f.get(dstGVR, "team", "app"))
}

func TestForward_SourceChangedBeforePruneIsKept(t *testing.T) {
	src := sourceCR("app", spec())
	src.SetResourceVersion("1")
	f := newFakeKube(src)

	// Someone updates the source after we read it.
	newer := src.DeepCopy()
	newer.SetResourceVersion("2")
	_ = unstructured.SetNestedField(newer.Object, "app:2", "spec", "image")
	_, err := f.dyn.Resource(srcGVR).Namespace("team").Update(context.Background(), newer, metav1UpdateOptions())
	require.NoError(t, err)

	res := forward(t, f, src, true, false)
	require.Error(t, res.Err)
	assert.ErrorContains(t, res.Err, "source changed during migration")
	kept := f.get(srcGVR, "team", "app")
	require.NotNil(t, kept)
	assert.Equal(t, "app:2", kept.Object["spec"].(map[string]interface{})["image"])
}

// ── status ───────────────────────────────────────────────────────────────────

func withStatus(u *unstructured.Unstructured) *unstructured.Unstructured {
	u.Object["status"] = map[string]interface{}{"phase": "Ready"}
	return u
}

func TestForward_StatusNotCarriedByDefault(t *testing.T) {
	src := withStatus(sourceCR("app", spec()))
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	dst := f.get(dstGVR, "team", "app")
	_, has := dst.Object["status"]
	assert.False(t, has)

	snap, ok := snapshotSourceOf(t, dst)
	require.True(t, ok)
	assert.Equal(t, map[string]interface{}{"phase": "Ready"}, snap.Status, "snapshot keeps status for rollback")
}

func TestForward_CarryStatus(t *testing.T) {
	src := withStatus(sourceCR("app", spec()))
	f := newFakeKube(src)
	crd := kindCRD(false)
	crd.Migrate.To.CarryStatus = true

	res, _ := f.migrator(core.ModeForward, false).processForward(context.Background(), src, crd, f.dyn.Resource(dstGVR), false, nop)
	require.NoError(t, res.Err)
	assert.Equal(t, map[string]interface{}{"phase": "Ready"}, f.get(dstGVR, "team", "app").Object["status"])
}

func TestRollback_RestoresSourceStatus(t *testing.T) {
	src := withStatus(sourceCR("app", spec()))
	f := newFakeKube(src)
	forward(t, f, src, true, false)
	require.Nil(t, f.get(srcGVR, "team", "app"))

	res := rollbackAll(t, f, false)
	require.Len(t, res, 1)
	require.NoError(t, res[0].Err)
	assert.Equal(t, map[string]interface{}{"phase": "Ready"}, f.get(srcGVR, "team", "app").Object["status"])
}

func snapshotSourceOf(t *testing.T, dst *unstructured.Unstructured) (kindSnapshotSource, bool) {
	t.Helper()
	return snapshotSource(dst)
}

func TestForward_CollisionWithinRunCaughtInDryRun(t *testing.T) {
	a := sourceCR("app", spec())
	b := sourceCR("app", spec())
	b.SetNamespace("other")
	f := newFakeKube(a, b)
	m := f.migrator(core.ModeForward, true)
	// Both land on one name, as with a cluster-scoped destination.
	dst := &sameNamespace{f.dyn.Resource(dstGVR), "team"}

	// dstClusterScoped: both destinations are named "app".
	first, _ := m.processForward(context.Background(), a, kindCRD(false), dst, true, nop)
	require.NoError(t, first.Err)
	second, _ := m.processForward(context.Background(), b, kindCRD(false), dst, true, nop)
	require.Error(t, second.Err)
	assert.ErrorContains(t, second.Err, "is also the target of team/app in this run")
	assert.Nil(t, f.get(dstGVR, "team", "app"), "dry-run wrote nothing")
}

func TestRollback_ReportsSourceRef(t *testing.T) {
	src := sourceCR("app", spec())
	f := newFakeKube(src)
	forward(t, f, src, false, false)

	res := rollbackAll(t, f, true)
	require.Len(t, res, 1)
	assert.Equal(t, "team/app", res[0].Ref())
}
