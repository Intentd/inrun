package kind

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func source(namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "old.io/v1",
		"kind":       "Old",
		"spec":       map[string]interface{}{"replicas": int64(2)},
	}}
	u.SetName("app")
	u.SetNamespace(namespace)
	u.SetLabels(map[string]string{"team": "a"})
	u.SetAnnotations(map[string]string{
		"note":                    "keep",
		"kubectl.kubernetes.io/x": "drop",
		"inrun.dev/y":             "drop",
	})
	return u
}

func crdTo(to types.CRMigrateTarget) *types.CRDEntry {
	if to.APIVersion == "" {
		to.APIVersion = "new.io/v1"
	}
	if to.Kind == "" {
		to.Kind = "New"
	}
	return &types.CRDEntry{Migrate: &types.CRMigrate{To: to}}
}

func testMigrator() *migrator { return newMigrator(&core.Runner{}) }

func TestBuildDestination_NamespacedToNamespaced(t *testing.T) {
	dst, err := testMigrator().buildDestination(source("team"), crdTo(types.CRMigrateTarget{}), false)
	require.NoError(t, err)
	assert.Equal(t, "new.io/v1", dst.GetAPIVersion())
	assert.Equal(t, "New", dst.GetKind())
	assert.Equal(t, "team", dst.GetNamespace())
	assert.Equal(t, "app", dst.GetName())
	assert.Equal(t, map[string]interface{}{"replicas": int64(2)}, dst.Object["spec"])
}

func TestBuildDestination_ClusterToNamespacedUsesToNamespace(t *testing.T) {
	dst, err := testMigrator().buildDestination(source(""), crdTo(types.CRMigrateTarget{Namespace: "platform"}), false)
	require.NoError(t, err)
	assert.Equal(t, "platform", dst.GetNamespace())
}

func TestBuildDestination_NamespacedToClusterDropsNamespace(t *testing.T) {
	dst, err := testMigrator().buildDestination(source("team"), crdTo(types.CRMigrateTarget{Namespace: "ignored"}), true)
	require.NoError(t, err)
	assert.Equal(t, "", dst.GetNamespace())
}

func TestBuildDestination_SpecIsCopied(t *testing.T) {
	src := source("team")
	dst, err := testMigrator().buildDestination(src, crdTo(types.CRMigrateTarget{}), false)
	require.NoError(t, err)
	dst.Object["spec"].(map[string]interface{})["replicas"] = int64(9)
	assert.Equal(t, int64(2), src.Object["spec"].(map[string]interface{})["replicas"])
}

func TestBuildDestination_PreserveDefaultKeepsUserMetaOnly(t *testing.T) {
	to := types.CRMigrateTarget{Metadata: types.CRMigrateMetadata{Labels: map[string]string{"team": "b", "tier": "gold"}}}
	dst, err := testMigrator().buildDestination(source("team"), crdTo(to), false)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"team": "b", "tier": "gold"}, dst.GetLabels())
	ann := dst.GetAnnotations()
	assert.Equal(t, "keep", ann["note"])
	assert.NotContains(t, ann, "kubectl.kubernetes.io/x")
	assert.NotContains(t, ann, "inrun.dev/y")
	assert.Contains(t, ann, labels.AnnotationMigrateKindSnapshot)
	assert.Contains(t, ann, labels.AnnotationMigrateTimestamp)
}

func TestBuildDestination_PreserveFalseUsesDeclaredOnly(t *testing.T) {
	to := types.CRMigrateTarget{Metadata: types.CRMigrateMetadata{
		PreserveSource: ptr.To(false),
		Labels:         map[string]string{"tier": "gold"},
	}}
	dst, err := testMigrator().buildDestination(source("team"), crdTo(to), false)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"tier": "gold"}, dst.GetLabels())
	assert.NotContains(t, dst.GetAnnotations(), "note")
}

func TestBuildDestination_TemplateResolved(t *testing.T) {
	to := types.CRMigrateTarget{Metadata: types.CRMigrateMetadata{Labels: map[string]string{"from": "{{ .metadata.name }}"}}}
	dst, err := testMigrator().buildDestination(source("team"), crdTo(to), false)
	require.NoError(t, err)
	assert.Equal(t, "app", dst.GetLabels()["from"])
}

func TestBuildDestination_TemplateFailureIsError(t *testing.T) {
	to := types.CRMigrateTarget{Metadata: types.CRMigrateMetadata{Labels: map[string]string{"bad": "{{ .metadata.name }"}}}
	_, err := testMigrator().buildDestination(source("team"), crdTo(to), false)
	require.Error(t, err)
	assert.ErrorContains(t, err, `label "bad"`)
}

func TestBuildDestination_SnapshotTooLarge(t *testing.T) {
	src := source("team")
	src.Object["spec"] = map[string]interface{}{"blob": strings.Repeat("x", maxAnnotationBytes)}
	_, err := testMigrator().buildDestination(src, crdTo(types.CRMigrateTarget{}), false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "snapshot too large")
}

func TestSnapshotSource_Is(t *testing.T) {
	src := source("team")
	dst, err := testMigrator().buildDestination(src, crdTo(types.CRMigrateTarget{}), false)
	require.NoError(t, err)

	owner, ok := snapshotSource(dst)
	require.True(t, ok)
	assert.True(t, owner.is(src))
	assert.False(t, owner.is(source("other")))
	assert.Equal(t, "Old team/app", owner.ref())
}

func TestSnapshotSource_None(t *testing.T) {
	_, ok := snapshotSource(source("team"))
	assert.False(t, ok)
}
