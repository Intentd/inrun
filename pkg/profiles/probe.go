package profiles

import "github.com/intentd/inrun/pkg/types"

// ProbeProfile is a named set of Kubernetes probe timing parameters.
//
//   - fast       — quick detection, low tolerance. HTTP APIs that start fast.
//   - standard   — balanced defaults for most web services.
//   - patient    — tolerant of slow operations. Batch workers.
//   - slow-start — 5-minute startup window for JVM apps, databases.
type ProbeProfile string

const (
	ProbeFast      ProbeProfile = "fast"
	ProbeStandard  ProbeProfile = "standard"
	ProbePatient   ProbeProfile = "patient"
	ProbeSlowStart ProbeProfile = "slow-start"
)

// ProbeTimings holds the Kubernetes probe timing parameters for a profile.
type ProbeTimings struct {
	InitialDelaySeconds int32
	PeriodSeconds       int32
	FailureThreshold    int32
	SuccessThreshold    int32
	TimeoutSeconds      int32
}

// DefaultProbeTimings is used when no profile is declared or the profile
// name is not recognized. Matches the standard profile values.
var DefaultProbeTimings = ProbeTimings{
	InitialDelaySeconds: 15,
	PeriodSeconds:       20,
	FailureThreshold:    3,
	SuccessThreshold:    1,
	TimeoutSeconds:      10,
}

var probeTimings = map[ProbeProfile]ProbeTimings{
	ProbeFast:      {InitialDelaySeconds: 5, PeriodSeconds: 10, FailureThreshold: 2, SuccessThreshold: 1, TimeoutSeconds: 5},
	ProbeStandard:  {InitialDelaySeconds: 15, PeriodSeconds: 20, FailureThreshold: 3, SuccessThreshold: 1, TimeoutSeconds: 10},
	ProbePatient:   {InitialDelaySeconds: 30, PeriodSeconds: 30, FailureThreshold: 5, SuccessThreshold: 1, TimeoutSeconds: 10},
	ProbeSlowStart: {InitialDelaySeconds: 0, PeriodSeconds: 10, FailureThreshold: 30, SuccessThreshold: 1, TimeoutSeconds: 10},
}

// ApplyProbeProfile returns the ProbeTimings for the named profile.
// User-defined profiles in reg are checked first; falls back to built-ins.
// The second return value is false when the name is not recognized — callers
// should fall back to DefaultProbeTimings in that case.
func ApplyProbeProfile(name string, reg *types.ProfileRegistry) (ProbeTimings, bool) {
	if reg != nil {
		if def, found := reg.LookupProbe(name); found {
			return ProbeTimings{
				InitialDelaySeconds: def.InitialDelaySeconds,
				PeriodSeconds:       def.PeriodSeconds,
				FailureThreshold:    def.FailureThreshold,
				SuccessThreshold:    def.SuccessThreshold,
				TimeoutSeconds:      def.TimeoutSeconds,
			}, true
		}
	}
	t, ok := probeTimings[ProbeProfile(name)]
	return t, ok
}

// IsValidProbeProfile reports whether name is a recognized probe profile.
func IsValidProbeProfile(name string) bool {
	_, ok := probeTimings[ProbeProfile(name)]
	return ok
}
