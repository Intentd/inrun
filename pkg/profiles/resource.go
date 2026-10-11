package profiles

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// ResourceProfile is a named CPU/memory preset.
type ResourceProfile string

const (
	ResourceTiny         ResourceProfile = "tiny"
	ResourceSmall        ResourceProfile = "small"
	ResourceMedium       ResourceProfile = "medium"
	ResourceLarge        ResourceProfile = "large"
	ResourceBurst        ResourceProfile = "burst"
	ResourceSteady       ResourceProfile = "steady"
	ResourceComputeHeavy ResourceProfile = "compute-heavy"
	ResourceMemoryHeavy  ResourceProfile = "memory-heavy"
)

// ApplyResourceProfile expands a named resource profile into a complete
// ResourceRequirements. User-defined profiles in reg are checked first; falls
// back to built-ins. Returns an error for unknown profile names.
func ApplyResourceProfile(name string, reg *types.ProfileRegistry) (*types.ResourceRequirements, error) {
	if reg != nil {
		if def, found := reg.LookupResource(name); found {
			return &types.ResourceRequirements{
				Requests: def.Requests,
				Limits:   def.Limits,
			}, nil
		}
	}
	switch ResourceProfile(strings.ToLower(name)) {
	case ResourceTiny:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "25m", "memory": "64Mi"},
			Limits:   map[string]string{"cpu": "100m", "memory": "128Mi"},
		}, nil
	case ResourceSmall:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
			Limits:   map[string]string{"cpu": "500m", "memory": "512Mi"},
		}, nil
	case ResourceMedium:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "250m", "memory": "256Mi"},
			Limits:   map[string]string{"cpu": "1", "memory": "1Gi"},
		}, nil
	case ResourceLarge:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "500m", "memory": "512Mi"},
			Limits:   map[string]string{"cpu": "2", "memory": "2Gi"},
		}, nil
	case ResourceBurst:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "200m", "memory": "256Mi"},
			Limits:   map[string]string{"cpu": "2", "memory": "2Gi"},
		}, nil
	case ResourceSteady:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "300m", "memory": "256Mi"},
			Limits:   map[string]string{"cpu": "600m", "memory": "512Mi"},
		}, nil
	case ResourceComputeHeavy:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "1", "memory": "512Mi"},
			Limits:   map[string]string{"cpu": "2", "memory": "1Gi"},
		}, nil
	case ResourceMemoryHeavy:
		return &types.ResourceRequirements{
			Requests: map[string]string{"cpu": "250m", "memory": "1Gi"},
			Limits:   map[string]string{"cpu": "500m", "memory": "2Gi"},
		}, nil
	default:
		return nil, fmt.Errorf("unknown resource profile: %q — built-ins: tiny, small, medium, large, burst, steady, compute-heavy, memory-heavy", name)
	}
}

// IsValidResourceProfile reports whether name is a recognized resource profile.
func IsValidResourceProfile(name string) bool {
	switch ResourceProfile(strings.ToLower(name)) {
	case ResourceTiny, ResourceSmall, ResourceMedium, ResourceLarge,
		ResourceBurst, ResourceSteady, ResourceComputeHeavy, ResourceMemoryHeavy:
		return true
	default:
		return false
	}
}
