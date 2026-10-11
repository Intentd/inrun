package profiles

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// ResourceQuotaProfile is a named namespace resource quota preset.
type ResourceQuotaProfile string

const (
	QuotaSmall  ResourceQuotaProfile = "small"
	QuotaMedium ResourceQuotaProfile = "medium"
	QuotaLarge  ResourceQuotaProfile = "large"
	QuotaXLarge ResourceQuotaProfile = "xlarge"
)

// ResourceQuotaLimits is a fully expanded set of hard limits.
type ResourceQuotaLimits struct {
	Hard map[string]string
}

// ApplyResourceQuotaProfile expands a named quota profile into a hard limits map.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyResourceQuotaProfile(name string, reg *types.ProfileRegistry) (*ResourceQuotaLimits, error) {
	if reg != nil {
		if def, found := reg.LookupResourceQuota(name); found {
			return &ResourceQuotaLimits{Hard: def.Hard}, nil
		}
	}
	switch ResourceQuotaProfile(strings.ToLower(name)) {
	case QuotaSmall:
		return &ResourceQuotaLimits{Hard: map[string]string{
			"pods":            "10",
			"cpu":             "2",
			"memory":          "4Gi",
			"requests.cpu":    "1",
			"requests.memory": "2Gi",
			"limits.cpu":      "2",
			"limits.memory":   "4Gi",
		}}, nil
	case QuotaMedium:
		return &ResourceQuotaLimits{Hard: map[string]string{
			"pods":            "20",
			"cpu":             "4",
			"memory":          "8Gi",
			"requests.cpu":    "2",
			"requests.memory": "4Gi",
			"limits.cpu":      "4",
			"limits.memory":   "8Gi",
		}}, nil
	case QuotaLarge:
		return &ResourceQuotaLimits{Hard: map[string]string{
			"pods":            "50",
			"cpu":             "8",
			"memory":          "16Gi",
			"requests.cpu":    "4",
			"requests.memory": "8Gi",
			"limits.cpu":      "8",
			"limits.memory":   "16Gi",
		}}, nil
	case QuotaXLarge:
		return &ResourceQuotaLimits{Hard: map[string]string{
			"pods":            "100",
			"cpu":             "16",
			"memory":          "32Gi",
			"requests.cpu":    "8",
			"requests.memory": "16Gi",
			"limits.cpu":      "16",
			"limits.memory":   "32Gi",
		}}, nil
	default:
		return nil, fmt.Errorf("unknown resourcequota profile: %q — allowed: small, medium, large, xlarge", name)
	}
}

// IsValidResourceQuotaProfile reports whether name is a recognized quota profile.
func IsValidResourceQuotaProfile(name string) bool {
	switch ResourceQuotaProfile(strings.ToLower(name)) {
	case QuotaSmall, QuotaMedium, QuotaLarge, QuotaXLarge:
		return true
	default:
		return false
	}
}
