package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
)

func TestPDBProfiles(t *testing.T) {
	cases := []struct {
		profile string
		wantMin string
		wantMax string
	}{
		{profile: "zero-downtime", wantMin: "100%", wantMax: ""},
		{profile: "rolling", wantMin: "", wantMax: "1"},
		{profile: "relaxed", wantMin: "", wantMax: "25%"},
	}

	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			result, err := profiles.ApplyPDBProfile(tc.profile, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.MinAvailable != tc.wantMin {
				t.Errorf("MinAvailable: got %q, want %q", result.MinAvailable, tc.wantMin)
			}
			if result.MaxUnavailable != tc.wantMax {
				t.Errorf("MaxUnavailable: got %q, want %q", result.MaxUnavailable, tc.wantMax)
			}
		})
	}
}

func TestPDBProfileMutualExclusivity(t *testing.T) {
	for _, name := range []string{"zero-downtime", "rolling", "relaxed"} {
		result, _ := profiles.ApplyPDBProfile(name, nil)
		if result.MinAvailable != "" && result.MaxUnavailable != "" {
			t.Errorf("profile %q sets both MinAvailable and MaxUnavailable", name)
		}
	}
}

func TestPDBProfileCaseInsensitive(t *testing.T) {
	for _, name := range []string{"ZERO-DOWNTIME", "Zero-Downtime", "ROLLING", "RELAXED"} {
		_, err := profiles.ApplyPDBProfile(name, nil)
		if err != nil {
			t.Errorf("profile %q: unexpected error: %v", name, err)
		}
	}
}

func TestPDBProfileUnknown(t *testing.T) {
	_, err := profiles.ApplyPDBProfile("unknown", nil)
	if err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

func TestIsValidPDBProfile(t *testing.T) {
	valid := []string{"zero-downtime", "rolling", "relaxed", "ROLLING", "RELAXED"}
	for _, name := range valid {
		if !profiles.IsValidPDBProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "strict", "moderate", "safe", "web"}
	for _, name := range invalid {
		if profiles.IsValidPDBProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
