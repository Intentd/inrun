package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestHPAProfiles(t *testing.T) {
	cases := []struct {
		profile        string
		wantCPU        int32
		wantUpWindow   int32
		wantUpPolicy   string
		wantUpCount    int
		wantDownWindow int32
		wantDownPolicy string
		wantDownCount  int
	}{
		{
			profile:        "web",
			wantCPU:        70,
			wantUpWindow:   0,
			wantUpPolicy:   "Max",
			wantUpCount:    2,
			wantDownWindow: 300,
			wantDownPolicy: "Min",
			wantDownCount:  1,
		},
		{
			profile:        "api",
			wantCPU:        60,
			wantUpWindow:   0,
			wantUpPolicy:   "Max",
			wantUpCount:    2,
			wantDownWindow: 600,
			wantDownPolicy: "Min",
			wantDownCount:  1,
		},
		{
			profile:        "latency-sensitive",
			wantCPU:        50,
			wantUpWindow:   0,
			wantUpPolicy:   "Max",
			wantUpCount:    2,
			wantDownWindow: 900,
			wantDownPolicy: "Min",
			wantDownCount:  1,
		},
		{
			profile:        "batch",
			wantCPU:        80,
			wantUpWindow:   30,
			wantUpPolicy:   "Max",
			wantUpCount:    1,
			wantDownWindow: 120,
			wantDownPolicy: "Min",
			wantDownCount:  1,
		},
		{
			profile:        "cost-optimized",
			wantCPU:        80,
			wantUpWindow:   180,
			wantUpPolicy:   "Min",
			wantUpCount:    1,
			wantDownWindow: 60,
			wantDownPolicy: "Max",
			wantDownCount:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			result, err := profiles.ApplyHPAProfile(tc.profile, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.CPUTarget != tc.wantCPU {
				t.Errorf("CPUTarget: got %d, want %d", result.CPUTarget, tc.wantCPU)
			}

			up := result.Behavior.ScaleUp
			if up == nil {
				t.Fatal("ScaleUp is nil")
			}
			assertRules(t, "scaleUp", up, tc.wantUpWindow, tc.wantUpPolicy, tc.wantUpCount)

			down := result.Behavior.ScaleDown
			if down == nil {
				t.Fatal("ScaleDown is nil")
			}
			assertRules(t, "scaleDown", down, tc.wantDownWindow, tc.wantDownPolicy, tc.wantDownCount)
		})
	}
}

func assertRules(t *testing.T, label string, r *types.HPAScalingRules, wantWindow int32, wantPolicy string, wantPolicies int) {
	t.Helper()
	if r.StabilizationWindowSeconds != wantWindow {
		t.Errorf("%s stabilizationWindowSeconds: got %d, want %d", label, r.StabilizationWindowSeconds, wantWindow)
	}
	if r.SelectPolicy != wantPolicy {
		t.Errorf("%s selectPolicy: got %q, want %q", label, r.SelectPolicy, wantPolicy)
	}
	if len(r.Policies) != wantPolicies {
		t.Errorf("%s policies count: got %d, want %d", label, len(r.Policies), wantPolicies)
	}
}

func TestHPAProfileCaseInsensitive(t *testing.T) {
	for _, name := range []string{"WEB", "Web", "API", "Latency-Sensitive", "BATCH", "Cost-Optimized"} {
		_, err := profiles.ApplyHPAProfile(name, nil)
		if err != nil {
			t.Errorf("profile %q: unexpected error: %v", name, err)
		}
	}
}

func TestHPAProfileUnknown(t *testing.T) {
	_, err := profiles.ApplyHPAProfile("unknown-profile", nil)
	if err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

func TestIsValidHPAProfile(t *testing.T) {
	valid := []string{"web", "api", "latency-sensitive", "batch", "cost-optimized", "WEB", "API", "BATCH"}
	for _, name := range valid {
		if !profiles.IsValidHPAProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "fast", "slow", "standard", "aggressive", "xlarge"}
	for _, name := range invalid {
		if profiles.IsValidHPAProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestHPAProfilePoliciesNonZero(t *testing.T) {
	for _, name := range []string{"web", "api", "latency-sensitive", "batch", "cost-optimized"} {
		result, _ := profiles.ApplyHPAProfile(name, nil)
		for i, p := range result.Behavior.ScaleUp.Policies {
			if p.Value == 0 {
				t.Errorf("profile %q scaleUp policy[%d]: Value is 0", name, i)
			}
			if p.PeriodSeconds == 0 {
				t.Errorf("profile %q scaleUp policy[%d]: PeriodSeconds is 0", name, i)
			}
		}
		for i, p := range result.Behavior.ScaleDown.Policies {
			if p.Value == 0 {
				t.Errorf("profile %q scaleDown policy[%d]: Value is 0", name, i)
			}
			if p.PeriodSeconds == 0 {
				t.Errorf("profile %q scaleDown policy[%d]: PeriodSeconds is 0", name, i)
			}
		}
	}
}
