// pkg/resources/shared/resource.go
package shared

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"

	"github.com/intentd/inrun/pkg/logger"
)

// ResolveResources resolves resource requirements: if resources.preset is set
// it expands to explicit requests/limits; otherwise the block is returned as-is.
// Returns nil when neither preset nor explicit values are declared.
func ResolveResources(r *types.ResourceRequirements, reg presets.Registry) *types.ResourceRequirements {
	if r == nil {
		return nil
	}
	if r.Preset != "" {
		p, ok := reg.Resource(r.Preset)
		if !ok {
			logger.Warn().Str("preset", r.Preset).Msg("unknown resources preset — skipping")
			return nil
		}
		return p.Requirements()
	}
	if len(r.Requests) == 0 && len(r.Limits) == 0 {
		return nil
	}
	return r
}

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
