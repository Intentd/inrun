package reconcile_test

import (
	"context"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

// passFixture is a one-CRD Catalog, an informer holding default/a, and a fake kube.
func passFixture(t *testing.T) (reconcile.Deps, contract.RegistryEntry) {
	t.Helper()
	cat := catalog.NewCatalogForTest(map[string]types.CRDEntry{"app": {
		APITypes: types.APITypes{Group: "demo.inrun.dev", Version: "v1", Kind: "App", Plural: "apps"},
		OperatorBox: &types.OperatorBoxConfig{Reconcile: &types.ReconcileConfig{
			Args: map[string]interface{}{"image": "{{ .spec.image }}"},
		}},
	}})
	crd := cat.EnabledCRDs()["app"]

	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, &unstructured.Unstructured{}, 0, cache.Indexers{})
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "demo.inrun.dev/v1",
		"kind":       "App",
		"metadata":   map[string]interface{}{"name": "a", "namespace": "default"},
		"spec":       map[string]interface{}{"image": "nginx"},
	}}
	require.NoError(t, inf.GetIndexer().Add(obj))

	deps := reconcile.Deps{Kube: simulate.NewFakeKubeclient(runtime.NewScheme()), Event: event.Discard(), Cat: cat}
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
