package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestProbeProfiles(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		ok      bool
		want    profiles.ProbeTimings
	}{
		{
			name:    "fast",
			profile: "fast",
			ok:      true,
			want:    profiles.ProbeTimings{InitialDelaySeconds: 5, PeriodSeconds: 10, FailureThreshold: 2, SuccessThreshold: 1, TimeoutSeconds: 5},
		},
		{
			name:    "standard",
			profile: "standard",
			ok:      true,
			want:    profiles.ProbeTimings{InitialDelaySeconds: 15, PeriodSeconds: 20, FailureThreshold: 3, SuccessThreshold: 1, TimeoutSeconds: 10},
		},
		{
			name:    "patient",
			profile: "patient",
			ok:      true,
			want:    profiles.ProbeTimings{InitialDelaySeconds: 30, PeriodSeconds: 30, FailureThreshold: 5, SuccessThreshold: 1, TimeoutSeconds: 10},
		},
		{
			name:    "slow-start",
			profile: "slow-start",
			ok:      true,
			want:    profiles.ProbeTimings{InitialDelaySeconds: 0, PeriodSeconds: 10, FailureThreshold: 30, SuccessThreshold: 1, TimeoutSeconds: 10},
		},
		{
			name:    "unknown returns false",
			profile: "turbo",
			ok:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := profiles.ApplyProbeProfile(tt.profile, nil)
			if ok != tt.ok {
				t.Fatalf("ok: want %v got %v", tt.ok, ok)
			}
			if ok && got != tt.want {
				t.Errorf("timings: want %+v got %+v", tt.want, got)
			}
		})
	}
}

func TestDefaultProbeTimingsMatchStandard(t *testing.T) {
	standard, ok := profiles.ApplyProbeProfile("standard", nil)
	if !ok {
		t.Fatal("standard profile not found")
	}
	if profiles.DefaultProbeTimings != standard {
		t.Errorf("DefaultProbeTimings does not match standard profile: default=%+v standard=%+v",
			profiles.DefaultProbeTimings, standard)
	}
}

func TestProbeProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		Probes: []types.ProbeProfileDef{
			{Name: "aggressive", InitialDelaySeconds: 2, PeriodSeconds: 5, FailureThreshold: 1, SuccessThreshold: 1, TimeoutSeconds: 3},
		},
	}

	timings, ok := profiles.ApplyProbeProfile("aggressive", &reg)
	if !ok {
		t.Fatal("expected user-defined profile to be found")
	}
	if timings.InitialDelaySeconds != 2 {
		t.Errorf("initialDelaySeconds: want 2 got %d", timings.InitialDelaySeconds)
	}
	if timings.PeriodSeconds != 5 {
		t.Errorf("periodSeconds: want 5 got %d", timings.PeriodSeconds)
	}
}

func TestProbeProfileUserDefinedOverridesBuiltIn(t *testing.T) {
	reg := types.ProfileRegistry{
		Probes: []types.ProbeProfileDef{
			{Name: "fast", InitialDelaySeconds: 99, PeriodSeconds: 99},
		},
	}
	timings, ok := profiles.ApplyProbeProfile("fast", &reg)
	if !ok {
		t.Fatal("expected profile to be found")
	}
	if timings.InitialDelaySeconds != 99 {
		t.Errorf("user-defined should override built-in: want 99 got %d", timings.InitialDelaySeconds)
	}
}

func TestIsValidProbeProfile(t *testing.T) {
	valid := []string{"fast", "standard", "patient", "slow-start"}
	for _, name := range valid {
		if !profiles.IsValidProbeProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "turbo", "FAST"}
	for _, name := range invalid {
		if profiles.IsValidProbeProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
