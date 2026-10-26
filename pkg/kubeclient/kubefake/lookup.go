package kubefake

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
)

// The store (see store.go) is the one place an object lives. These read it
// and write it directly, not through a client, so they record no op: they
// are the harness's own access, not the reconciler's.

// Object returns the stored state of one object, as a GET would after the
// simulation, without recording an op. Returns a NotFound error when the
// store does not hold it.
func (f *Kubeclient) Object(_ context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	tracker := f.tracker
	if _, ok := builtinKind(gvr); ok {
		tracker = f.clientset.(*fake.Clientset).Tracker()
	}
	obj, err := tracker.Get(gvr, namespace, name)
	if err != nil {
		return nil, err
	}
	return toUnstructured(obj)
}

// CatchUp brings the informer's copy of namespace/name up to the store, as
// a real informer catches up with the API server between reconciles: what
// any write did to the object — a part patch or a plain patch through a
// client — is what the next cycle prepares from. A typed copy stays typed.
func (f *Kubeclient) CatchUp(ctx context.Context, inf cache.SharedIndexInformer, gvr schema.GroupVersionResource, namespace, name string) error {
	if inf == nil {
		return nil
	}
	stored, err := f.Object(ctx, gvr, namespace, name)
	if err != nil {
		return nil // deleted: the informer's copy stays, as nothing replaces it
	}
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	idx := inf.GetIndexer()
	next := runtime.Object(stored)
	if old, exists, _ := idx.GetByKey(key); exists {
		if _, isU := old.(*unstructured.Unstructured); !isU {
			if typed, ok := old.(runtime.Object); ok {
				fresh := reflect.New(reflect.TypeOf(typed).Elem()).Interface().(runtime.Object)
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(stored.Object, fresh); err != nil {
					return fmt.Errorf("catching up %s: %w", key, err)
				}
				next = fresh
			}
		}
	}
	return idx.Update(next)
}

// toUnstructured converts a stored object, typed or not, to a copy with
// apiVersion and kind set: typed objects in a tracker carry no TypeMeta.
func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.DeepCopy(), nil
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return nil, fmt.Errorf("converting %T: %w", obj, err)
	}
	u := &unstructured.Unstructured{Object: m}
	if u.GetKind() == "" {
		if kinds, _, err := clientgoscheme.Scheme.ObjectKinds(obj); err == nil && len(kinds) > 0 {
			u.SetGroupVersionKind(kinds[0])
		}
	}
	return u, nil
}
