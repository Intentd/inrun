package apply

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
)

var (
	deployGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	svcGVR    = schema.GroupVersionResource{Version: "v1", Resource: "services"}
	pvcGVR    = schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}

	pruneManaged = []domain.ManagedResource{{Kind: "deployment"}, {Kind: "service"}, {Kind: "persistentvolumeclaim"}}
)

// child builds a child object; uid is its owner-uid label, byInrun adds the
// inrun-reconciler field manager.
func child(apiVersion, kind, ns, name, uid string, byInrun bool, extra map[string]string) *unstructured.Unstructured {
	u := object.New(apiVersion, kind, name, object.InNamespace(ns))
	lbls := map[string]string{}
	if uid != "" {
		lbls[labels.OwnerUID] = uid
	}
	for k, v := range extra {
		lbls[k] = v
	}
	u.SetLabels(lbls)
	if byInrun {
		u.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: FieldManager, Operation: metav1.ManagedFieldsOperationApply}})
	} else {
		u.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationUpdate}})
	}
	return u
}

func pruneClient(objs ...runtime.Object) *kubefake.Dynamic {
	return kubefake.NewDynamic(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		deployGVR: "DeploymentList",
		svcGVR:    "ServiceList",
		pvcGVR:    "PersistentVolumeClaimList",
	}, objs...)
}

func names(t *testing.T, dyn *kubefake.Dynamic, gvr schema.GroupVersionResource) []string {
	t.Helper()
	list, err := dyn.Resource(gvr).Namespace("").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	var out []string
	for _, i := range list.Items {
		out = append(out, i.GetName())
	}
	sort.Strings(out)
	return out
}

func TestPrune(t *testing.T) {
	owner := ownerObj("app", "default") // UID "test-uid"
	keepDeploy := child("apps/v1", "Deployment", "default", "app", "test-uid", true, nil)

	fixture := func() *kubefake.Dynamic {
		return pruneClient(
			keepDeploy.DeepCopy(),
			child("apps/v1", "Deployment", "default", "old", "test-uid", true, nil),           // dropped from the Result
			child("v1", "Service", "other-ns", "old-svc", "test-uid", true, nil),              // dropped, other namespace
			child("apps/v1", "Deployment", "default", "someone-else", "other-uid", true, nil), // another CR's child
			child("apps/v1", "Deployment", "default", "manual", "test-uid", false, nil),       // not applied by Inrun
			child("apps/v1", "Deployment", "default", "protected", "test-uid", true,
				map[string]string{labels.DeletionProtectionLabel: "true"}),
			child("v1", "PersistentVolumeClaim", "default", "data", "test-uid", true, nil),
		)
	}
	applied := []*unstructured.Unstructured{keepDeploy}

	t.Run("off", func(t *testing.T) {
		dyn := fixture()
		pruned, err := Prune(context.Background(), dyn, owner, pruneManaged, nil, applied)
		require.NoError(t, err)
		assert.Empty(t, pruned)
		assert.Len(t, names(t, dyn, deployGVR), 5)
	})

	t.Run("deletes only this CR's Inrun-applied children the Result dropped", func(t *testing.T) {
		dyn := fixture()
		pruned, err := Prune(context.Background(), dyn, owner, pruneManaged, &types.PruneConfig{Enabled: true}, applied)
		require.NoError(t, err)
		assert.ElementsMatch(t, []Pruned{
			{Kind: "Deployment", Namespace: "default", Name: "old"},
			{Kind: "Service", Namespace: "other-ns", Name: "old-svc"},
			{Kind: "PersistentVolumeClaim", Namespace: "default", Name: "data"},
		}, pruned)
		assert.Equal(t, []string{"app", "manual", "protected", "someone-else"}, names(t, dyn, deployGVR))
		assert.Empty(t, names(t, dyn, svcGVR))
	})

	t.Run("keep spares a kind", func(t *testing.T) {
		dyn := fixture()
		_, err := Prune(context.Background(), dyn, owner, pruneManaged,
			&types.PruneConfig{Enabled: true, Keep: []string{"pvc"}}, applied)
		require.NoError(t, err)
		assert.Equal(t, []string{"data"}, names(t, dyn, pvcGVR))
	})

	t.Run("an empty Result prunes nothing unless allowEmpty", func(t *testing.T) {
		dyn := fixture()
		pruned, err := Prune(context.Background(), dyn, owner, pruneManaged, &types.PruneConfig{Enabled: true}, nil)
		require.NoError(t, err)
		assert.Empty(t, pruned)

		pruned, err = Prune(context.Background(), dyn, owner, pruneManaged, &types.PruneConfig{Enabled: true, AllowEmpty: true}, nil)
		require.NoError(t, err)
		assert.Len(t, pruned, 4) // app, old, old-svc, data
	})
}

// The final sweep takes every child carrying the owner's UID, whoever wrote
// it (a template child has no inrun-reconciler field manager), in any
// namespace, but not another owner's, a kept kind or a protected object.
func TestPruneChildren_EveryChildOfTheOwner(t *testing.T) {
	protected := map[string]string{labels.DeletionProtectionLabel: "true"}
	dyn := pruneClient(
		child("apps/v1", "Deployment", "team", "from-result", "uid-1", true, nil),
		child("apps/v1", "Deployment", "other-ns", "from-template", "uid-1", false, nil),
		child("v1", "PersistentVolumeClaim", "team", "data", "uid-1", false, nil),
		child("v1", "Service", "team", "protected", "uid-1", false, protected),
		child("apps/v1", "Deployment", "team", "someone-elses", "uid-2", true, nil),
	)
	owner := object.Empty(object.WithUID("uid-1"))

	deleted, err := PruneChildren(context.Background(), dyn, owner,
		[]schema.GroupVersionResource{deployGVR, svcGVR, pvcGVR}, []string{"persistentvolumeclaim"})
	require.NoError(t, err)
	assert.Len(t, deleted, 2)
	assert.Equal(t, []string{"someone-elses"}, names(t, dyn, deployGVR))
	assert.Equal(t, []string{"data"}, names(t, dyn, pvcGVR), "kept kind")
	assert.Equal(t, []string{"protected"}, names(t, dyn, svcGVR), "deletion-protected")
}

// The surface sweep narrows the owner's children to one serve target.
func TestPruneOwned_SurfaceSelector(t *testing.T) {
	prev := map[string]string{labels.InrunServeTarget: "small"}
	cur := map[string]string{labels.InrunServeTarget: "large"}
	dyn := pruneClient(
		child("apps/v1", "Deployment", "team", "old", "uid-1", false, prev),
		child("apps/v1", "Deployment", "team", "new", "uid-1", false, cur),
	)
	sel := labels.OwnerUID + "=uid-1," + labels.InrunServeTarget + "=small"
	_, err := PruneOwned(context.Background(), dyn, sel, []schema.GroupVersionResource{deployGVR}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, names(t, dyn, deployGVR))

	_, err = PruneOwned(context.Background(), dyn, labels.InrunServeTarget+"=large", []schema.GroupVersionResource{deployGVR}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, names(t, dyn, deployGVR), "a selector without an owner deletes nothing")
}
