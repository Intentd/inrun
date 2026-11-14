package domain

import (
	"github.com/intentd/tools/kubeclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// Object is a Kubernetes API object: metadata and type information. It is the
// same type as the client's, so values pass between them unchanged.
type Object = kubeclient.Object

// ObjectList is a Kubernetes API list: list metadata and type information.
type ObjectList = kubeclient.ObjectList

// ObjectKey identifies a Kubernetes object by namespace and name.
type ObjectKey = kubeclient.ObjectKey

// ToDomainObject unwraps a cache tombstone and asserts to domain.Object.
func ToDomainObject(obj interface{}) (Object, bool) {
	d, ok := UnwrapCacheTombstone(obj).(Object)
	return d, ok
}

// UnwrapCacheTombstone unwraps a cache tombstone and returns the object.
func UnwrapCacheTombstone(obj interface{}) interface{} {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	return obj
}

// Raw returns the object's underlying field map without copying.
// Callers that need to mutate must call DeepCopy first.
func Raw(obj Object) map[string]interface{} {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.Object
	}
	return nil
}

// ToUnstructured unwraps a cache tombstone and converts to *unstructured.Unstructured.
// Works for both dynamic informers (which store *unstructured.Unstructured) and typed
// informers (which store scheme-registered concrete types such as *v1alpha1.WebApp).
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, bool) {
	d, ok := ToDomainObject(obj)
	if !ok {
		return nil, false
	}
	if u, ok := d.(*unstructured.Unstructured); ok {
		return u, true
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		return nil, false
	}
	return &unstructured.Unstructured{Object: m}, true
}

// Copy returns a deep copy of obj that the caller may modify.
func Copy(obj Object) Object { return obj.DeepCopyObject().(Object) }

// FromCache is ToUnstructured for an object from an informer's cache: it
// returns a copy, since the informer reads the cached one concurrently.
func FromCache(obj interface{}) (*unstructured.Unstructured, bool) {
	u, ok := ToUnstructured(obj)
	if !ok {
		return nil, false
	}
	return u.DeepCopy(), true
}
