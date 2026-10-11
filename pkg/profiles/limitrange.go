package profiles

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// ApplyLimitRangeProfile expands a named LimitRange profile into a list of LimitRangeItems.
// User-defined profiles in reg are checked first; there are no built-in LimitRange profiles.
// Returns an error for unknown profile names.
func ApplyLimitRangeProfile(name string, reg *types.ProfileRegistry) ([]types.LimitRangeItem, error) {
	if reg != nil {
		if def, found := reg.LookupLimitRange(name); found {
			return def.Limits, nil
		}
	}
	return nil, fmt.Errorf("unknown limitrange profile: %q — define it in profiles.limitRanges", name)
}

// IsValidLimitRangeProfile reports whether name is a recognized LimitRange profile.
// LimitRange profiles are always user-defined — there are no built-in presets.
func IsValidLimitRangeProfile(name string, reg *types.ProfileRegistry) bool {
	if reg == nil {
		return false
	}
	_, found := reg.LookupLimitRange(name)
	return found
}
