package kind

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/migrate/core"
	"github.com/intentd/inrun/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var (
	srcGVK = schema.GroupVersionKind{Group: "old.io", Version: "v1", Kind: "Old"}
	dstGVK = schema.GroupVersionKind{Group: "new.io", Version: "v1", Kind: "New"}
	srcGVR = schema.GroupVersionResource{Group: "old.io", Version: "v1", Resource: "olds"}
	dstGVR = schema.GroupVersionResource{Group: "new.io", Version: "v1", Resource: "news"}
)

// fakeKube is the shared fake client with the kind migrator's dynamic client
// and mapper swapped in, so the tests can shape what the "server" stores.
type fakeKube struct {
	*kubefake.Kubeclient
	dyn    *kubefake.Dynamic
	mapper meta.RESTMapper
	// shape, when set, changes what the "server" stores on apply, to
	// simulate defaulting or schema pruning.
	shape func(*unstructured.Unstructured)
}

func newFakeKube(objs ...*unstructured.Unstructured) *fakeKube {
	seed := make([]runtime.Object, len(objs))
	for i, o := range objs {
		seed[i] = o
	}
	dyn := kubefake.NewDynamic(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{srcGVR: "OldList", dstGVR: "NewList"}, seed...)

	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(srcGVK, meta.RESTScopeNamespace)
	mapper.Add(dstGVK, meta.RESTScopeNamespace)

	f := &fakeKube{Kubeclient: kubefake.New(runtime.NewScheme()), dyn: dyn, mapper: mapper}
	tracker := dyn.Tracker()

	// Server-side apply: store the object (create or replace), honour dry-run.
	dyn.PrependReactor("patch", "*", func(a kubefake.Action) (bool, runtime.Object, error) {
		pa := a.(kubefake.PatchActionImpl)
		if pa.GetPatchType() != k8stypes.ApplyPatchType {
			return false, nil, nil
		}
		u := object.Empty()
		if err := u.UnmarshalJSON(pa.GetPatch()); err != nil {
			return true, nil, err
		}
		if f.shape != nil {
			f.shape(u)
		}
		if len(pa.GetPatchOptions().DryRun) > 0 {
			return true, u, nil
		}
		gvr := pa.GetResource()
		if _, err := tracker.Get(gvr, pa.GetNamespace(), pa.GetName()); apierrors.IsNotFound(err) {
			return true, u, tracker.Create(gvr, u, pa.GetNamespace())
		}
		return true, u, tracker.Update(gvr, u, pa.GetNamespace())
	})
	// Delete: honour preconditions like the API server, then dry-run.
	dyn.PrependReactor("delete", "*", func(a kubefake.Action) (bool, runtime.Object, error) {
		da := a.(kubefake.DeleteActionImpl)
		opts := da.GetDeleteOptions()
		if p := opts.Preconditions; p != nil && p.ResourceVersion != nil {
			cur, err := tracker.Get(da.GetResource(), da.GetNamespace(), da.GetName())
			if err == nil && cur.(metav1.Object).GetResourceVersion() != *p.ResourceVersion {
				return true, nil, apierrors.NewConflict(da.GetResource().GroupResource(), da.GetName(), fmt.Errorf("resourceVersion changed"))
			}
		}
		return len(opts.DryRun) > 0, nil, nil
	})
	dyn.PrependReactor("create", "*", func(a kubefake.Action) (bool, runtime.Object, error) {
		ca := a.(kubefake.CreateActionImpl)
		if len(ca.GetCreateOptions().DryRun) > 0 {
			return true, ca.GetObject(), nil
		}
		return false, nil, nil
	})
	return f
}

func (f *fakeKube) DynamicClient() dynamic.Interface { return f.dyn }
func (f *fakeKube) RESTMapper() meta.RESTMapper      { return f.mapper }

func (f *fakeKube) Delete(ctx context.Context, obj domain.Object, opts metav1.DeleteOptions) error {
	u := obj.(*unstructured.Unstructured)
	m, err := f.mapper.RESTMapping(u.GroupVersionKind().GroupKind(), u.GroupVersionKind().Version)
	if err != nil {
		return err
	}
	return f.dyn.Resource(m.Resource).Namespace(u.GetNamespace()).Delete(ctx, u.GetName(), opts)
}

// get returns the stored object, or nil.
func (f *fakeKube) get(gvr schema.GroupVersionResource, ns, name string) *unstructured.Unstructured {
	u, err := f.dyn.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return u
}

func (f *fakeKube) migrator(mode core.Mode, dryRun bool) *migrator {
	return newMigrator(&core.Runner{Kube: f, Mode: mode, DryRun: dryRun})
}

// sourceCR builds a source CR in namespace "team".
func sourceCR(name string, spec map[string]interface{}) *unstructured.Unstructured {
	return object.New(srcGVK.GroupVersion().String(), srcGVK.Kind, name, object.InNamespace("team"),
		object.WithSpec(spec), object.WithLabels(map[string]string{"app": "demo"}))
}

// kindCRD is the source CRD entry, migrating Old → New.
func kindCRD(prune bool) *types.CRDEntry {
	return &types.CRDEntry{
		APITypes: types.APITypes{Group: srcGVK.Group, Version: srcGVK.Version, Kind: srcGVK.Kind, Plural: "olds"},
		Migrate: &types.CRMigrate{
			To:    types.CRMigrateTarget{APIVersion: dstGVK.GroupVersion().String(), Kind: dstGVK.Kind, Plural: "news"},
			Prune: prune,
		},
	}
}

// sameNamespace routes every namespace to one, to simulate many sources
// landing on one cluster-scoped destination name.
type sameNamespace struct {
	dynamic.NamespaceableResourceInterface
	ns string
}

func (s *sameNamespace) Namespace(string) dynamic.ResourceInterface {
	return s.NamespaceableResourceInterface.Namespace(s.ns)
}

func metav1UpdateOptions() metav1.UpdateOptions { return metav1.UpdateOptions{} }
