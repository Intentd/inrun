package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestResourceProfiles(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		expectErr  bool
		cpuRequest string
		memRequest string
		cpuLimit   string
		memLimit   string
	}{
		{"tiny", "tiny", false, "25m", "64Mi", "100m", "128Mi"},
		{"small", "small", false, "100m", "128Mi", "500m", "512Mi"},
		{"medium", "medium", false, "250m", "256Mi", "1", "1Gi"},
		{"large", "large", false, "500m", "512Mi", "2", "2Gi"},
		{"burst", "burst", false, "200m", "256Mi", "2", "2Gi"},
		{"steady", "steady", false, "300m", "256Mi", "600m", "512Mi"},
		{"compute-heavy", "compute-heavy", false, "1", "512Mi", "2", "1Gi"},
		{"memory-heavy", "memory-heavy", false, "250m", "1Gi", "500m", "2Gi"},
		{"unknown profile fails fast", "xlarge", true, "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := profiles.ApplyResourceProfile(tt.profile, nil)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := r.Requests["cpu"]; got != tt.cpuRequest {
				t.Errorf("requests.cpu: want %q got %q", tt.cpuRequest, got)
			}
			if got := r.Requests["memory"]; got != tt.memRequest {
				t.Errorf("requests.memory: want %q got %q", tt.memRequest, got)
			}
			if got := r.Limits["cpu"]; got != tt.cpuLimit {
				t.Errorf("limits.cpu: want %q got %q", tt.cpuLimit, got)
			}
			if got := r.Limits["memory"]; got != tt.memLimit {
				t.Errorf("limits.memory: want %q got %q", tt.memLimit, got)
			}
		})
	}
}

func TestResourceProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		Resources: []types.ResourceProfileDef{
			{
				Name:     "gpu-worker",
				Requests: map[string]string{"cpu": "4", "memory": "8Gi"},
				Limits:   map[string]string{"cpu": "8", "memory": "16Gi"},
			},
		},
	}

	r, err := profiles.ApplyResourceProfile("gpu-worker", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Requests["cpu"]; got != "4" {
		t.Errorf("requests.cpu: want 4 got %q", got)
	}
	if got := r.Limits["memory"]; got != "16Gi" {
		t.Errorf("limits.memory: want 16Gi got %q", got)
	}
}

func TestResourceProfileUserDefinedOverridesBuiltIn(t *testing.T) {
	reg := types.ProfileRegistry{
		Resources: []types.ResourceProfileDef{
			{
				Name:     "small",
				Requests: map[string]string{"cpu": "999m", "memory": "999Mi"},
				Limits:   map[string]string{"cpu": "999m", "memory": "999Mi"},
			},
		},
	}
	r, err := profiles.ApplyResourceProfile("small", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Requests["cpu"]; got != "999m" {
		t.Errorf("user-defined should override built-in: want 999m got %q", got)
	}
}

func TestIsValidResourceProfile(t *testing.T) {
	valid := []string{"tiny", "small", "medium", "large", "burst", "steady", "compute-heavy", "memory-heavy"}
	for _, name := range valid {
		if !profiles.IsValidResourceProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	for _, name := range []string{"MEDIUM", "Tiny", "BURST"} {
		if !profiles.IsValidResourceProfile(name) {
			t.Errorf("expected case-insensitive %q to be valid", name)
		}
	}

	invalid := []string{"", "xlarge", "nano"}
	for _, name := range invalid {
		if profiles.IsValidResourceProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
