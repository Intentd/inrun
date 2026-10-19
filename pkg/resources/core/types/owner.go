package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Owner is the object a resource belongs to: it gets the owner reference and
// lends its namespace when the spec names none. Any Kubernetes object
// satisfies it, typed or unstructured.
type Owner interface {
	metav1.Object
	runtime.Object
}
