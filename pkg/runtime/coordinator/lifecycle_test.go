package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
type lifecycleKube struct {
	kubeclient.Interface
	cs        *k8sfake.Clientset
	dyn       *dynfake.FakeDynamicClient
	annotated map[string]string
}

func (k *lifecycleKube) Clientset() kubernetes.Interface  { return k.cs }
func (k *lifecycleKube) DynamicClient() dynamic.Interface { return k.dyn }
func (k *lifecycleKube) PatchAnnotations(_ context.Context, _ runtime.Object, a map[string]string, _ metav1.PatchOptions) error {
	k.annotated = a
	return nil
}

var cmGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

func newLifecycleKube(typed []runtime.Object, dyn ...runtime.Object) *lifecycleKube {
	return &lifecycleKube{
		cs: k8sfake.NewClientset(typed...),
		dyn: dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
			map[schema.GroupVersionResource]string{cmGVR: "ConfigMapList"}, dyn...),
	}
}

func deployment(name, ownerKey string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{labels.InrunOwner: ownerKey},
	}}
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

func TestSweepPreviousSurface_RemovesWhatTheOldTargetCreated(t *testing.T) {
	kube := newLifecycleKube(
		[]runtime.Object{deployment("old", "app.a"), deployment("new", "app.b")},
		configMap("elsewhere", "old-config", map[string]string{labels.InrunOwner: "app.a"}, true),
	)
	obj := cr(map[string]string{labels.AnnotationServeAlias: "b", labels.AnnotationLastSurface: "a"})

	require.NoError(t, sweepPreviousSurface(context.Background(), kube, lifecycleCRD, obj))

	deps, err := kube.cs.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, deps.Items, 1)
	assert.Equal(t, "new", deps.Items[0].Name)

	cms, err := kube.dyn.Resource(cmGVR).Namespace("").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, cms.Items, "a managed kind in another namespace is swept too")
	assert.Equal(t, "b", kube.annotated[labels.AnnotationLastSurface])
}

func TestSweepPreviousSurface_NothingToDo(t *testing.T) {
	for name, ann := range map[string]map[string]string{
		"no target":      nil,
		"target already": {labels.AnnotationServeAlias: "a", labels.AnnotationLastSurface: "a"},
	} {
		kube := newLifecycleKube([]runtime.Object{deployment("old", "app.a")})
		require.NoError(t, sweepPreviousSurface(context.Background(), kube, lifecycleCRD, cr(ann)), name)
		deps, _ := kube.cs.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, deps.Items, 1, name)
		assert.Nil(t, kube.annotated, name)
	}
}

func TestDeleteChildren_RemovesResultChildrenOwnerReferencesMiss(t *testing.T) {
	kube := newLifecycleKube(nil,
		configMap("elsewhere", "mine", map[string]string{labels.OwnerUID: "app-uid"}, true),
		configMap("elsewhere", "someone-elses", map[string]string{labels.OwnerUID: "other-uid"}, true),
	)
	prepared := &domain.PreparedRequest{Object: cr(nil)}

	deleted, err := deleteChildren(context.Background(), kube, lifecycleCRD, prepared)
	require.NoError(t, err)
	assert.Equal(t, []apply.Pruned{{Kind: "ConfigMap", Namespace: "elsewhere", Name: "mine"}}, deleted)
}
