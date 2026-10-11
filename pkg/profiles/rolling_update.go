package profiles

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// RollingUpdateProfile is a named Deployment rolling update strategy preset.
//
//   - safe       — maxSurge: 1, maxUnavailable: 0. Adds one pod before removing one.
//     Zero capacity drop during rollout. Default for production services.
//   - fast       — maxSurge: 25%, maxUnavailable: 25%. Removes and adds pods in parallel.
//     Faster rollout with brief capacity reduction. Kubernetes default behaviour.
//   - blue-green — maxSurge: 100%, maxUnavailable: 0. Full duplicate capacity during rollout,
//     then removes old pods. Most expensive but cleanest cutover.
type RollingUpdateProfile string

const (
	RollingUpdateSafe      RollingUpdateProfile = "safe"
	RollingUpdateFast      RollingUpdateProfile = "fast"
	RollingUpdateBlueGreen RollingUpdateProfile = "blue-green"
)

// RollingUpdateProfileResult is returned by ApplyRollingUpdateProfile.
type RollingUpdateProfileResult struct {
	// MaxSurge — maximum pods above desired replica count during rollout.
	// Integer string ("1") or percentage string ("25%").
	MaxSurge string

	// MaxUnavailable — maximum pods that can be unavailable during rollout.
	// Integer string ("0") or percentage string ("25%").
	MaxUnavailable string
}

// ApplyRollingUpdateProfile expands a named rolling update profile into MaxSurge and MaxUnavailable.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyRollingUpdateProfile(name string, reg *types.ProfileRegistry) (RollingUpdateProfileResult, error) {
	if reg != nil {
		if def, found := reg.LookupRollingUpdate(name); found {
			return RollingUpdateProfileResult{MaxSurge: def.MaxSurge, MaxUnavailable: def.MaxUnavailable}, nil
		}
	}
	switch RollingUpdateProfile(strings.ToLower(name)) {
	case RollingUpdateSafe:
		return RollingUpdateProfileResult{MaxSurge: "1", MaxUnavailable: "0"}, nil
	case RollingUpdateFast:
		return RollingUpdateProfileResult{MaxSurge: "25%", MaxUnavailable: "25%"}, nil
	case RollingUpdateBlueGreen:
		return RollingUpdateProfileResult{MaxSurge: "100%", MaxUnavailable: "0"}, nil
	default:
		return RollingUpdateProfileResult{}, fmt.Errorf(
			"unknown rolling update profile: %q — allowed: safe, fast, blue-green", name,
		)
	}
}

// IsValidRollingUpdateProfile reports whether name is a recognized rolling update profile.
func IsValidRollingUpdateProfile(name string) bool {
	switch RollingUpdateProfile(strings.ToLower(name)) {
	case RollingUpdateSafe, RollingUpdateFast, RollingUpdateBlueGreen:
		return true
	default:
		return false
	}
}
