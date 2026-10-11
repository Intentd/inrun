package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestContainerSecurityProfiles(t *testing.T) {
	tests := []struct {
		name             string
		profile          string
		expectErr        bool
		allowPrivEsc     *bool
		runAsNonRoot     *bool
		readOnlyRootFS   *bool
		dropCapabilities []string
	}{
		{
			name:             "baseline",
			profile:          "baseline",
			allowPrivEsc:     boolPtr(false),
			dropCapabilities: []string{"NET_RAW"},
		},
		{
			name:             "restricted",
			profile:          "restricted",
			allowPrivEsc:     boolPtr(false),
			runAsNonRoot:     boolPtr(true),
			dropCapabilities: []string{"ALL"},
		},
		{
			name:             "hardened",
			profile:          "hardened",
			allowPrivEsc:     boolPtr(false),
			runAsNonRoot:     boolPtr(true),
			readOnlyRootFS:   boolPtr(true),
			dropCapabilities: []string{"ALL"},
		},
		{
			name:      "unknown profile fails fast",
			profile:   "permissive",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := profiles.ApplyContainerSecurityProfile(tt.profile, nil)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ptrBoolEqual(sc.AllowPrivilegeEscalation, tt.allowPrivEsc) {
				t.Errorf("AllowPrivilegeEscalation: want %v got %v", tt.allowPrivEsc, sc.AllowPrivilegeEscalation)
			}
			if !ptrBoolEqual(sc.RunAsNonRoot, tt.runAsNonRoot) {
				t.Errorf("RunAsNonRoot: want %v got %v", tt.runAsNonRoot, sc.RunAsNonRoot)
			}
			if !ptrBoolEqual(sc.ReadOnlyRootFilesystem, tt.readOnlyRootFS) {
				t.Errorf("ReadOnlyRootFilesystem: want %v got %v", tt.readOnlyRootFS, sc.ReadOnlyRootFilesystem)
			}
			if len(tt.dropCapabilities) > 0 {
				if sc.Capabilities == nil {
					t.Fatal("expected capabilities block but got nil")
				}
				if len(sc.Capabilities.Drop) != len(tt.dropCapabilities) || sc.Capabilities.Drop[0] != tt.dropCapabilities[0] {
					t.Errorf("capabilities.drop: want %v got %v", tt.dropCapabilities, sc.Capabilities.Drop)
				}
			}
		})
	}
}

func TestPodSecurityProfiles(t *testing.T) {
	tests := []struct {
		name         string
		profile      string
		expectErr    bool
		runAsNonRoot *bool
		runAsUser    *int64
		runAsGroup   *int64
		fsGroup      *int64
	}{
		{
			name:         "baseline",
			profile:      "baseline",
			runAsNonRoot: boolPtr(false),
		},
		{
			name:         "restricted",
			profile:      "restricted",
			runAsNonRoot: boolPtr(true),
			runAsUser:    int64Ptr(1000),
		},
		{
			name:         "hardened",
			profile:      "hardened",
			runAsNonRoot: boolPtr(true),
			runAsUser:    int64Ptr(65534),
			runAsGroup:   int64Ptr(65534),
			fsGroup:      int64Ptr(65534),
		},
		{
			name:      "unknown profile fails fast",
			profile:   "permissive",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := profiles.ApplyPodSecurityProfile(tt.profile, nil)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ptrBoolEqual(ps.RunAsNonRoot, tt.runAsNonRoot) {
				t.Errorf("RunAsNonRoot: want %v got %v", tt.runAsNonRoot, ps.RunAsNonRoot)
			}
			if !ptrInt64Equal(ps.RunAsUser, tt.runAsUser) {
				t.Errorf("RunAsUser: want %v got %v", tt.runAsUser, ps.RunAsUser)
			}
			if !ptrInt64Equal(ps.RunAsGroup, tt.runAsGroup) {
				t.Errorf("RunAsGroup: want %v got %v", tt.runAsGroup, ps.RunAsGroup)
			}
			if !ptrInt64Equal(ps.FSGroup, tt.fsGroup) {
				t.Errorf("FSGroup: want %v got %v", tt.fsGroup, ps.FSGroup)
			}
		})
	}
}

func TestContainerSecurityProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		ContainerSecurity: []types.ContainerSecurityProfileDef{
			{
				Name:                     "strict-readonly",
				AllowPrivilegeEscalation: boolPtr(false),
				ReadOnlyRootFilesystem:   boolPtr(true),
				RunAsNonRoot:             boolPtr(true),
				Capabilities:             &types.CapabilitiesConfig{Drop: []string{"ALL"}},
			},
		},
	}

	sc, err := profiles.ApplyContainerSecurityProfile("strict-readonly", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ptrBoolEqual(sc.ReadOnlyRootFilesystem, boolPtr(true)) {
		t.Errorf("ReadOnlyRootFilesystem: want true got %v", sc.ReadOnlyRootFilesystem)
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) == 0 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("expected capabilities.drop=[ALL], got %v", sc.Capabilities)
	}
}

func TestPodSecurityProfileUserDefined(t *testing.T) {
	uid := int64(2000)
	reg := types.ProfileRegistry{
		PodSecurity: []types.PodSecurityProfileDef{
			{Name: "ci-runner", RunAsNonRoot: boolPtr(true), RunAsUser: &uid},
		},
	}

	ps, err := profiles.ApplyPodSecurityProfile("ci-runner", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ptrBoolEqual(ps.RunAsNonRoot, boolPtr(true)) {
		t.Errorf("RunAsNonRoot: want true got %v", ps.RunAsNonRoot)
	}
	if !ptrInt64Equal(ps.RunAsUser, &uid) {
		t.Errorf("RunAsUser: want 2000 got %v", ps.RunAsUser)
	}
}

func TestIsValidSecurityProfile(t *testing.T) {
	valid := []string{"baseline", "restricted", "hardened"}
	for _, name := range valid {
		if !profiles.IsValidSecurityProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	for _, name := range []string{"HARDENED", "Baseline", "RESTRICTED"} {
		if !profiles.IsValidSecurityProfile(name) {
			t.Errorf("expected case-insensitive %q to be valid", name)
		}
	}

	invalid := []string{"", "permissive"}
	for _, name := range invalid {
		if profiles.IsValidSecurityProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func boolPtr(v bool) *bool    { return &v }
func int64Ptr(v int64) *int64 { return &v }

func ptrBoolEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrInt64Equal(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
