package reconcile_test

import (
	"context"
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// recordingReconciler returns result and remembers the request it was given.
type recordingReconciler struct {
	result domain.Result
	got    *domain.Request
}

func (r *recordingReconciler) Reconcile(_ context.Context, req domain.Request) (domain.Result, error) {
	r.got = &req
	return r.result, nil
}

// passFixture is a one-CRD Blueprint, an informer holding default/a, and a
// fake kube whose store holds default/a too, as a cluster does.
func passFixture(t *testing.T) (reconcile.Deps, contract.RegistryEntry) {
	t.Helper()
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"app": {
		APITypes: types.APITypes{Group: "demo.inrun.dev", Version: "v1", Kind: "App", Plural: "apps"},
		OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
			Args: map[string]interface{}{"image": "{{ .spec.image }}"},
		}},
	}})
	crd := bp.EnabledCRDs()["app"]

	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, object.Empty(), 0, cache.Indexers{})
	obj := object.New("demo.inrun.dev/v1", "App", "a", object.InNamespace("default"),
		object.WithSpec(map[string]any{"image": "nginx"}))
	require.NoError(t, inf.GetIndexer().Add(obj))

	deps := reconcile.Deps{Kube: kubefake.New(runtime.NewScheme(), obj.DeepCopy()), Event: event.Discard(), Blueprint: bp}
	return deps, contract.RegistryEntry{CRD: crd, Informer: inf}
}

func TestRun_FullPass(t *testing.T) {
	deps, entry := passFixture(t)
	rec := &recordingReconciler{result: domain.Result{Requeue: &domain.Requeue{After: time.Minute, Reason: "poll"}}}

	out := reconcile.Run(context.Background(), deps, reconcile.Input{
		GVK: entry.CRD.GVKString(), Key: "default/a", Entry: entry,
		Reconciler: rec, Health: vitals.NewCRDHealth("app"),
	})

	require.NoError(t, out.Err)
	assert.False(t, out.Skipped)
	assert.Equal(t, time.Minute, out.Next, "the Result's requeue schedules the next pass")
	require.NotNil(t, rec.got, "the reconciler was called")
	assert.Equal(t, "nginx", rec.got.Args()["image"], "args are rendered before the reconciler runs")
	assert.False(t, rec.got.Finalizing())
}

func TestRun_ObjectGoneIsSkipped(t *testing.T) {
	deps, entry := passFixture(t)
	rec := &recordingReconciler{}

	out := reconcile.Run(context.Background(), deps, reconcile.Input{
		GVK: entry.CRD.GVKString(), Key: "default/missing", Entry: entry,
		Reconciler: rec, Health: vitals.NewCRDHealth("app"),
	})

	assert.NoError(t, out.Err)
	assert.True(t, out.Skipped)
	assert.Negative(t, out.Next)
	assert.Nil(t, rec.got, "nothing to reconcile")
}

// finalizingFixture is passFixture with default/a being deleted and the CRD
// declaring requeue; spec.waiting says whether the CR's cleanup is pending.
func finalizingFixture(t *testing.T, requeue *types.RequeueConfig, waiting string) (reconcile.Deps, contract.RegistryEntry) {
	t.Helper()
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"app": {
		APITypes:    types.APITypes{Group: "demo.inrun.dev", Version: "v1", Kind: "App", Plural: "apps"},
		OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{Requeue: requeue}},
	}})
	crd := bp.EnabledCRDs()["app"]

	obj := object.New("demo.inrun.dev/v1", "App", "a", object.InNamespace("default"),
		object.Deleting(time.Now()), object.WithFinalizers("inrun.dev/cleanup"),
		object.WithSpec(map[string]any{"waiting": waiting}))
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, object.Empty(), 0, cache.Indexers{})
	require.NoError(t, inf.GetIndexer().Add(obj))

	kube := kubefake.New(runtime.NewScheme())
	require.NoError(t, kube.Seed(obj.DeepCopy()))
	return reconcile.Deps{Kube: kube, Event: event.Discard(), Blueprint: bp}, contract.RegistryEntry{CRD: crd, Informer: inf}
}

func runFinalizing(t *testing.T, deps reconcile.Deps, entry contract.RegistryEntry) (reconcile.Outcome, *recordingReconciler) {
	t.Helper()
	rec := &recordingReconciler{}
	out := reconcile.Run(context.Background(), deps, reconcile.Input{
		GVK: entry.CRD.GVKString(), Key: "default/a", Entry: entry,
		Reconciler: rec, Health: vitals.NewCRDHealth("app"),
	})
	require.NotNil(t, rec.got, "the reconciler was called")
	assert.True(t, rec.got.Finalizing())
	return out, rec
}

func TestRun_Finalizing_ConditionalRequeueHolds(t *testing.T) {
	waitWhilePending := &types.RequeueConfig{
		After: "10s",
		When:  types.Conditions{{Field: "spec.waiting", Equals: "true"}},
	}

	deps, entry := finalizingFixture(t, waitWhilePending, "true")
	out, _ := runFinalizing(t, deps, entry)
	require.NoError(t, out.Err)
	assert.Equal(t, 10*time.Second, out.Next, "a conditional requeue that holds keeps the CR finalizing")

	deps, entry = finalizingFixture(t, waitWhilePending, "false")
	out, _ = runFinalizing(t, deps, entry)
	require.NoError(t, out.Err)
	assert.Negative(t, out.Next, "once the condition fails the CR is released")
}

func TestRun_Finalizing_BareRequeueDoesNotHold(t *testing.T) {
	deps, entry := finalizingFixture(t, &types.RequeueConfig{After: "10s"}, "true")
	out, _ := runFinalizing(t, deps, entry)
	require.NoError(t, out.Err)
	assert.Negative(t, out.Next, "a periodic requeue is a refresh, not a wait")
}

// A CR its validation rules deny is not reconciled, but its status says why:
// ValidationFailed=True with the rule's message, and Ready=False.
func TestRun_ValidationDenialWritesStatus(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"app": {
		APITypes: types.APITypes{Group: "demo.inrun.dev", Version: "v1", Kind: "App", Plural: "apps"},
		Admission: &types.AdmissionConfig{Validation: &types.ValidationConfig{Rules: []types.ValidationRule{{
			Field: "spec.image", Prefix: "approved/", Message: "images come from approved/",
			Action: types.ValidationActionDeny,
		}}}},
	}})
	crd := bp.EnabledCRDs()["app"]
	obj := object.New("demo.inrun.dev/v1", "App", "a", object.InNamespace("default"),
		object.WithSpec(map[string]any{"image": "nginx"}))
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, object.Empty(), 0, cache.Indexers{})
	require.NoError(t, inf.GetIndexer().Add(obj))
	// The store holds the CR, as a cluster does; the status patch lands there.
	kube := kubefake.New(runtime.NewScheme(), obj.DeepCopy())
	deps := reconcile.Deps{Kube: kube, Event: event.Discard(), Blueprint: bp}
	entry := contract.RegistryEntry{CRD: crd, Informer: inf}

	rec := &recordingReconciler{}
	out := reconcile.Run(context.Background(), deps, reconcile.Input{
		GVK: crd.GVKString(), Key: "default/a", Entry: entry,
		Reconciler: rec, Health: vitals.NewCRDHealth("app"),
	})

	require.Error(t, out.Err)
	assert.Nil(t, rec.got, "a denied CR is not reconciled")

	stored, err := kube.Object(context.Background(), schema.GroupVersionResource{Group: "demo.inrun.dev", Version: "v1", Resource: "apps"}, "default", "a")
	require.NoError(t, err)
	conditions, _, _ := unstructured.NestedSlice(stored.Object, "status", "conditions")
	byType := map[string]map[string]any{}
	for _, c := range conditions {
		m := c.(map[string]any)
		byType[m["type"].(string)] = m
	}
	require.Contains(t, byType, "ValidationFailed")
	assert.Equal(t, "True", byType["ValidationFailed"]["status"])
	assert.Contains(t, byType["ValidationFailed"]["message"], "images come from approved/")
	if ready, ok := byType["Ready"]; assert.True(t, ok, "Ready condition") {
		assert.Equal(t, "False", ready["status"])
	}
}
