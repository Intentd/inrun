package profiles

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// PDBProfile is a named PodDisruptionBudget disruption limit preset.
//
//   - zero-downtime — minAvailable: 100%. No pod may be voluntarily disrupted.
//     Use for stateful services, databases, anything where partial availability is unacceptable.
//   - rolling       — maxUnavailable: 1. Exactly one pod at a time. Safe default for production.
//   - relaxed       — maxUnavailable: 25%. Up to a quarter of pods may be disrupted.
//     Use for stateless services where brief capacity reduction is acceptable.
type PDBProfile string

const (
	PDBZeroDowntime PDBProfile = "zero-downtime"
	PDBRolling      PDBProfile = "rolling"
	PDBRelaxed      PDBProfile = "relaxed"
)

// PDBProfileResult is returned by ApplyPDBProfile.
type PDBProfileResult struct {
	// MinAvailable — minimum pods that must remain available.
	// Non-empty only for profiles that use minAvailable semantics.
	MinAvailable string

	// MaxUnavailable — maximum pods that may be unavailable.
	// Non-empty only for profiles that use maxUnavailable semantics.
	MaxUnavailable string
}

// ApplyPDBProfile expands a named PDB profile into disruption limit values.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyPDBProfile(name string, reg *types.ProfileRegistry) (PDBProfileResult, error) {
	if reg != nil {
		if def, found := reg.LookupPDB(name); found {
			return PDBProfileResult{MinAvailable: def.MinAvailable, MaxUnavailable: def.MaxUnavailable}, nil
		}
	}
	switch PDBProfile(strings.ToLower(name)) {
	case PDBZeroDowntime:
		return PDBProfileResult{MinAvailable: "100%"}, nil
	case PDBRolling:
		return PDBProfileResult{MaxUnavailable: "1"}, nil
	case PDBRelaxed:
		return PDBProfileResult{MaxUnavailable: "25%"}, nil
	default:
		return PDBProfileResult{}, fmt.Errorf(
			"unknown PDB profile: %q — allowed: zero-downtime, rolling, relaxed", name,
		)
	}
}

// IsValidPDBProfile reports whether name is a recognized PDB profile.
func IsValidPDBProfile(name string) bool {
	switch PDBProfile(strings.ToLower(name)) {
	case PDBZeroDowntime, PDBRolling, PDBRelaxed:
		return true
	default:
		return false
	}
}
