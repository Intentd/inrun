package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestResourceQuotaProfiles(t *testing.T) {
	var empty *types.ProfileRegistry

	tests := []struct {
		name      string
		profile   string
		expectErr bool
		pods      string
		cpu       string
		memory    string
	}{
		{"small", "small", false, "10", "2", "4Gi"},
		{"medium", "medium", false, "20", "4", "8Gi"},
		{"large", "large", false, "50", "8", "16Gi"},
		{"xlarge", "xlarge", false, "100", "16", "32Gi"},
		{"case-insensitive SMALL", "SMALL", false, "10", "2", "4Gi"},
		{"unknown profile fails fast", "nano", true, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := profiles.ApplyResourceQuotaProfile(tt.profile, empty)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := res.Hard["pods"]; got != tt.pods {
				t.Errorf("pods: want %q got %q", tt.pods, got)
			}
			if got := res.Hard["cpu"]; got != tt.cpu {
				t.Errorf("cpu: want %q got %q", tt.cpu, got)
			}
			if got := res.Hard["memory"]; got != tt.memory {
				t.Errorf("memory: want %q got %q", tt.memory, got)
			}
		})
	}
}

func TestResourceQuotaProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{
				Name: "ci",
				Hard: map[string]string{
					"pods":   "5",
					"cpu":    "1",
					"memory": "2Gi",
				},
			},
		},
	}

	res, err := profiles.ApplyResourceQuotaProfile("ci", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := res.Hard["pods"]; got != "5" {
		t.Errorf("pods: want 5 got %q", got)
	}
	if got := res.Hard["memory"]; got != "2Gi" {
		t.Errorf("memory: want 2Gi got %q", got)
	}
}

func TestResourceQuotaProfileUserDefinedOverridesBuiltIn(t *testing.T) {
	reg := types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "small", Hard: map[string]string{"pods": "999"}},
		},
	}
	res, err := profiles.ApplyResourceQuotaProfile("small", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := res.Hard["pods"]; got != "999" {
		t.Errorf("user-defined should override built-in: want 999 got %q", got)
	}
}

func TestIsValidResourceQuotaProfile(t *testing.T) {
	valid := []string{"small", "medium", "large", "xlarge"}
	for _, name := range valid {
		if !profiles.IsValidResourceQuotaProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	for _, name := range []string{"SMALL", "Medium", "LARGE"} {
		if !profiles.IsValidResourceQuotaProfile(name) {
			t.Errorf("expected case-insensitive %q to be valid", name)
		}
	}

	invalid := []string{"", "nano", "micro"}
	for _, name := range invalid {
		if profiles.IsValidResourceQuotaProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
