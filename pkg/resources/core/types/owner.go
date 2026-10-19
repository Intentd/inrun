package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// Owner is the object a resource belongs to: it gets the owner reference and
// lends its namespace when the spec names none. Any Kubernetes object
// satisfies it, typed or unstructured.
type Owner interface {
	metav1.Object
	runtime.Object
}

// OwnerReferences returns the controller owner reference to owner, which
// makes the object owner's: Kubernetes deletes it with owner.
func OwnerReferences(owner Owner) []metav1.OwnerReference {
	apiVersion := ""
	kind := ""
	if u, ok := owner.(*unstructured.Unstructured); ok {
		apiVersion = u.GetAPIVersion()
		kind = u.GetKind()
	} else {
		gvk := owner.GetObjectKind().GroupVersionKind()
		apiVersion = gvk.GroupVersion().String()
		kind = gvk.Kind
	}

	return []metav1.OwnerReference{
		{
			APIVersion:         apiVersion,
			Kind:               kind,
			Name:               owner.GetName(),
			UID:                owner.GetUID(),
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
	}
}
