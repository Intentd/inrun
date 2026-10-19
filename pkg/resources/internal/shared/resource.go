// pkg/resources/shared/resource.go
package shared

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// BuildResourceRequirements converts an Inrun ResourceRequirements spec into a
// Kubernetes corev1.ResourceRequirements object.
func BuildResourceRequirements(r *types.ResourceRequirements) corev1.ResourceRequirements {
	req := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	for k, v := range r.Requests {
		req.Requests[corev1.ResourceName(k)] = resource.MustParse(v)
	}
	for k, v := range r.Limits {
		req.Limits[corev1.ResourceName(k)] = resource.MustParse(v)
	}
	return req
}

// ResourceRequirementsEqual reports whether two ResourceRequirements are equivalent.
func ResourceRequirementsEqual(a, b corev1.ResourceRequirements) bool {
	return resourceListEqual(a.Requests, b.Requests) && resourceListEqual(a.Limits, b.Limits)
}

func resourceListEqual(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va.Cmp(vb) != 0 {
			return false
		}
	}
	return true
}
