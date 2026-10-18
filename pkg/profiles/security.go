package profiles

import (
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
)

// SecurityProfile is a named security preset.
//
//   - baseline   — prevents privilege escalation, drops NET_RAW.
//   - restricted — drops all capabilities, requires non-root. Matches the
//     Kubernetes "restricted" Pod Security Standard.
//   - hardened   — restricted plus read-only root filesystem.
type SecurityProfile string

const (
	SecurityBaseline   SecurityProfile = "baseline"
	SecurityRestricted SecurityProfile = "restricted"
	SecurityHardened   SecurityProfile = "hardened"
)

// ApplyContainerSecurityProfile expands a named profile into a
// ContainerSecurityContext. User-defined profiles in reg are checked first;
// falls back to built-ins. Returns an error for unknown profile names.
func ApplyContainerSecurityProfile(name string, reg *types.ProfileRegistry) (*types.ContainerSecurityContext, error) {
	if reg != nil {
		if def, found := reg.LookupContainerSecurity(name); found {
			return &types.ContainerSecurityContext{
				AllowPrivilegeEscalation: def.AllowPrivilegeEscalation,
				ReadOnlyRootFilesystem:   def.ReadOnlyRootFilesystem,
				RunAsNonRoot:             def.RunAsNonRoot,
				RunAsUser:                def.RunAsUser,
				RunAsGroup:               def.RunAsGroup,
				Capabilities:             def.Capabilities,
			}, nil
		}
	}
	switch SecurityProfile(strings.ToLower(name)) {
	case SecurityBaseline:
		return &types.ContainerSecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &types.CapabilitiesConfig{Drop: []string{"NET_RAW"}},
		}, nil
	case SecurityRestricted:
		return &types.ContainerSecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
			Capabilities:             &types.CapabilitiesConfig{Drop: []string{"ALL"}},
		}, nil
	case SecurityHardened:
		return &types.ContainerSecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			RunAsNonRoot:             ptr.To(true),
			Capabilities:             &types.CapabilitiesConfig{Drop: []string{"ALL"}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown container security profile: %q — allowed: baseline, restricted, hardened", name)
	}
}

// ApplyPodSecurityProfile expands a named profile into a PodSecurityContext.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyPodSecurityProfile(name string, reg *types.ProfileRegistry) (*types.PodSecurityContext, error) {
	if reg != nil {
		if def, found := reg.LookupPodSecurity(name); found {
			return &types.PodSecurityContext{
				RunAsNonRoot: def.RunAsNonRoot,
				RunAsUser:    def.RunAsUser,
				RunAsGroup:   def.RunAsGroup,
				FSGroup:      def.FSGroup,
			}, nil
		}
	}
	switch SecurityProfile(strings.ToLower(name)) {
	case SecurityBaseline:
		return &types.PodSecurityContext{
			RunAsNonRoot: ptr.To(false),
		}, nil
	case SecurityRestricted:
		return &types.PodSecurityContext{
			RunAsNonRoot: ptr.To(true),
			RunAsUser:    ptr.To[int64](1000),
		}, nil
	case SecurityHardened:
		return &types.PodSecurityContext{
			RunAsNonRoot: ptr.To(true),
			RunAsUser:    ptr.To[int64](65534),
			RunAsGroup:   ptr.To[int64](65534),
			FSGroup:      ptr.To[int64](65534),
		}, nil
	default:
		return nil, fmt.Errorf("unknown pod security profile: %q — allowed: baseline, restricted, hardened", name)
	}
}

// IsValidSecurityProfile reports whether name is a recognized security profile.
func IsValidSecurityProfile(name string) bool {
	switch SecurityProfile(strings.ToLower(name)) {
	case SecurityBaseline, SecurityRestricted, SecurityHardened:
		return true
	default:
		return false
	}
}
