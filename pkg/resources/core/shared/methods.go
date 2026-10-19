package shared

import (
	"github.com/intentd/inrun/pkg/resources/core/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

// ResolveNamespace — priority: spec.Namespace → owner namespace → "default"
func ResolveNamespace(owner types.Owner, namespace string) string {
	if namespace != "" {
		return namespace
	}
	if owner.GetNamespace() != "" {
		return owner.GetNamespace()
	}
	return "default"
}

// ToPullSecrets converts a slice of string to a []corev1.LocalObjectReference
// Acceptable as Pull secrets
func ToPullSecrets(names []string) []corev1.LocalObjectReference {
	out := make([]corev1.LocalObjectReference, len(names))
	for i, n := range names {
		out[i] = corev1.LocalObjectReference{Name: n}
	}
	return out
}

// ResolveOwnerReferences returns the common ownership reference for inrun-managed resources.
func ResolveOwnerReferences(owner types.Owner) []metav1.OwnerReference {
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
