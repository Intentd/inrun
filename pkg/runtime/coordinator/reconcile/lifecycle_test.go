package reconcile

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestFinalizeDeadline(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	deleting := func(ago time.Duration) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		ts := metav1.NewTime(now.Add(-ago))
		u.SetDeletionTimestamp(&ts)
		return u
	}
	thirty := &types.FinalizerConfig{Timeout: "30m"}

	expired, left := finalizeDeadline(deleting(10*time.Minute), nil, now)
	assert.False(t, expired, "no timeout set: never expires")
	assert.Zero(t, left)

	expired, left = finalizeDeadline(deleting(10*time.Minute), thirty, now)
	assert.False(t, expired)
	assert.Equal(t, 20*time.Minute, left)

	expired, _ = finalizeDeadline(deleting(31*time.Minute), thirty, now)
	assert.True(t, expired)

	expired, _ = finalizeDeadline(&unstructured.Unstructured{}, thirty, now)
	assert.False(t, expired, "not being deleted")
}

// lifecycleKube is the part of the kube client the lifecycle steps use: a
// typed and a dynamic fake, and annotation patches recorded.
var cmGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

func newLifecycleKube(objs ...runtime.Object) *kubefake.Kubeclient {
	return kubefake.New(runtime.NewScheme(), objs...)
}

// surfaceRecorded reports whether the last-surface annotation was patched.
func surfaceRecorded(kube *kubefake.Kubeclient) bool {
	for _, op := range kube.Ops() {
		if op.Verb == "patch" && op.Resource == "annotations" {
			return true
		}
	}
	return false
}

func configMap(ns, name string, lbls map[string]string, byInrun bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetNamespace(ns)
	u.SetName(name)
	u.SetLabels(lbls)
	if byInrun {
		u.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: apply.FieldManager, Operation: metav1.ManagedFieldsOperationApply}})
	}
	return u
}

func cr(annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetName("app")
	u.SetNamespace("default")
	u.SetUID("app-uid")
	u.SetAnnotations(annotations)
	return u
}

var lifecycleCRD = types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
	ManagedResources: []domain.ManagedResource{{Kind: "configmap"}},
}}}

// What the previous target created goes, in any namespace; the current
// target's children and another CR's of the same name stay.
func TestSweepPreviousSurface_RemovesWhatTheOldTargetCreated(t *testing.T) {
	target := func(uid, t string) map[string]string {
		return map[string]string{labels.OwnerUID: uid, labels.InrunServeTarget: t}
	}
	kube := newLifecycleKube(
		configMap("default", "old", target("app-uid", "a"), false),
		configMap("elsewhere", "old-elsewhere", target("app-uid", "a"), true),
		configMap("default", "new", target("app-uid", "b"), false),
		configMap("other-ns", "same-name-other-cr", target("other-uid", "a"), false),
	)
	obj := cr(map[string]string{labels.AnnotationServeAlias: "b", labels.AnnotationLastSurface: "a"})

	require.NoError(t, sweepPreviousSurface(context.Background(), kube, lifecycleCRD, obj))

	assert.ElementsMatch(t, []string{"new", "same-name-other-cr"}, cmNames(t, kube))
	assert.True(t, surfaceRecorded(kube))
	assert.Equal(t, "b", obj.GetAnnotations()[labels.AnnotationLastSurface])
}

func TestSweepPreviousSurface_NothingToDo(t *testing.T) {
	for name, ann := range map[string]map[string]string{
		"no target":      nil,
		"target already": {labels.AnnotationServeAlias: "a", labels.AnnotationLastSurface: "a"},
	} {
		kube := newLifecycleKube(configMap("default", "old",
			map[string]string{labels.OwnerUID: "app-uid", labels.InrunServeTarget: "a"}, false))
		require.NoError(t, sweepPreviousSurface(context.Background(), kube, lifecycleCRD, cr(ann)), name)
		assert.Equal(t, []string{"old"}, cmNames(t, kube), name)
		assert.False(t, surfaceRecorded(kube), name)
	}
}

func cmNames(t *testing.T, kube *kubefake.Kubeclient) []string {
	t.Helper()
	cms, err := kube.DynamicClient().Resource(cmGVR).Namespace("").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	var out []string
	for _, c := range cms.Items {
		out = append(out, c.GetName())
	}
	return out
}

func TestPruneChildren_RemovesResultChildrenOwnerReferencesMiss(t *testing.T) {
	kube := newLifecycleKube(
		configMap("elsewhere", "mine", map[string]string{labels.OwnerUID: "app-uid"}, true),
		configMap("elsewhere", "from-a-template", map[string]string{labels.OwnerUID: "app-uid"}, false),
		configMap("elsewhere", "someone-elses", map[string]string{labels.OwnerUID: "other-uid"}, true),
	)
	prepared := &domain.PreparedRequest{Object: cr(nil)}

	deleted, err := pruneChildren(context.Background(), kube, lifecycleCRD, prepared)
	require.NoError(t, err)
	assert.ElementsMatch(t, []apply.Pruned{
		{Kind: "ConfigMap", Namespace: "elsewhere", Name: "mine"},
		{Kind: "ConfigMap", Namespace: "elsewhere", Name: "from-a-template"},
	}, deleted, "whoever wrote it")
	assert.Equal(t, []string{"someone-elses"}, cmNames(t, kube))
}
