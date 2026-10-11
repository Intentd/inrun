package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
)

func TestRollingUpdateProfiles(t *testing.T) {
	cases := []struct {
		profile         string
		wantSurge       string
		wantUnavailable string
	}{
		{profile: "safe", wantSurge: "1", wantUnavailable: "0"},
		{profile: "fast", wantSurge: "25%", wantUnavailable: "25%"},
		{profile: "blue-green", wantSurge: "100%", wantUnavailable: "0"},
	}

	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			result, err := profiles.ApplyRollingUpdateProfile(tc.profile, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.MaxSurge != tc.wantSurge {
				t.Errorf("MaxSurge: got %q, want %q", result.MaxSurge, tc.wantSurge)
			}
			if result.MaxUnavailable != tc.wantUnavailable {
				t.Errorf("MaxUnavailable: got %q, want %q", result.MaxUnavailable, tc.wantUnavailable)
			}
		})
	}
}

func TestRollingUpdateSafeHasZeroUnavailable(t *testing.T) {
	result, _ := profiles.ApplyRollingUpdateProfile("safe", nil)
	if result.MaxUnavailable != "0" {
		t.Errorf("safe profile must have maxUnavailable=0 to guarantee zero capacity drop; got %q", result.MaxUnavailable)
	}
}

func TestRollingUpdateBlueGreenHasFullSurge(t *testing.T) {
	result, _ := profiles.ApplyRollingUpdateProfile("blue-green", nil)
	if result.MaxSurge != "100%" {
		t.Errorf("blue-green profile must have maxSurge=100%%; got %q", result.MaxSurge)
	}
	if result.MaxUnavailable != "0" {
		t.Errorf("blue-green profile must have maxUnavailable=0; got %q", result.MaxUnavailable)
	}
}

func TestRollingUpdateProfileCaseInsensitive(t *testing.T) {
	for _, name := range []string{"SAFE", "Safe", "FAST", "BLUE-GREEN", "Blue-Green"} {
		_, err := profiles.ApplyRollingUpdateProfile(name, nil)
		if err != nil {
			t.Errorf("profile %q: unexpected error: %v", name, err)
		}
	}
}

func TestRollingUpdateProfileUnknown(t *testing.T) {
	_, err := profiles.ApplyRollingUpdateProfile("canary", nil)
	if err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

func TestIsValidRollingUpdateProfile(t *testing.T) {
	valid := []string{"safe", "fast", "blue-green", "SAFE", "FAST", "BLUE-GREEN"}
	for _, name := range valid {
		if !profiles.IsValidRollingUpdateProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "canary", "rolling", "steady", "web"}
	for _, name := range invalid {
		if profiles.IsValidRollingUpdateProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
