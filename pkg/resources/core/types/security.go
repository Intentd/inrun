package types

// PodSecurityContext declares pod-level security settings.
// Set Preset for a named preset or declare individual fields directly.
// Preset and explicit fields are mutually exclusive.
type PodSecurityContext struct {
	// Preset — named security preset. One of: baseline, restricted, hardened.
	// Expands into the corresponding pod security fields at blueprint load time.
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`

	// RunAsNonRoot — if true, the pod's containers must run as a non-root user.
	RunAsNonRoot *bool `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`

	// RunAsUser — UID to run the entrypoint of all containers.
	RunAsUser *int64 `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`

	// RunAsGroup — GID to run the entrypoint of all containers.
	RunAsGroup *int64 `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`

	// FSGroup — supplemental GID applied to all containers.
	// Volumes owned by the pod will be owned by this GID.
	FSGroup *int64 `yaml:"fsGroup,omitempty" json:"fsGroup,omitempty"`
}

// HasMixedFields reports whether any explicit field is set
// alongside a preset — used to detect mixed usage.
func (p *PodSecurityContext) HasMixedFields() bool {
	if p == nil || p.Preset == "" {
		return false
	}
	return p.RunAsNonRoot != nil || p.RunAsUser != nil || p.RunAsGroup != nil || p.FSGroup != nil
}

// ContainerSecurityContext declares container-level security settings.
// Set Preset for a named preset or declare individual fields directly.
// Preset and explicit fields are mutually exclusive.
type ContainerSecurityContext struct {
	// Preset — named security preset. One of: baseline, restricted, hardened.
	// Expands into the corresponding container security fields at blueprint load time.
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`

	// AllowPrivilegeEscalation — controls whether a process can gain more
	// privileges than its parent process. Defaults to true if unset.
	AllowPrivilegeEscalation *bool `yaml:"allowPrivilegeEscalation,omitempty" json:"allowPrivilegeEscalation,omitempty"`

	// ReadOnlyRootFilesystem — mounts the container's root filesystem as read-only.
	ReadOnlyRootFilesystem *bool `yaml:"readOnlyRootFilesystem,omitempty" json:"readOnlyRootFilesystem,omitempty"`

	// RunAsNonRoot — if true, the container must run as a non-root user.
	RunAsNonRoot *bool `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`

	// RunAsUser — UID to run the container entrypoint.
	RunAsUser *int64 `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`

	// RunAsGroup — GID to run the container entrypoint.
	RunAsGroup *int64 `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`

	// Capabilities — Linux capabilities to add or drop from the container.
	Capabilities *CapabilitiesConfig `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// HasMixedFields reports whether any explicit field is set alongside a preset.
func (c *ContainerSecurityContext) HasMixedFields() bool {
	if c == nil || c.Preset == "" {
		return false
	}
	return c.AllowPrivilegeEscalation != nil ||
		c.ReadOnlyRootFilesystem != nil ||
		c.RunAsNonRoot != nil ||
		c.RunAsUser != nil ||
		c.RunAsGroup != nil ||
		c.Capabilities != nil
}

// CapabilitiesConfig declares Linux capabilities to add or drop.
type CapabilitiesConfig struct {
	// Add — capabilities to add beyond the default set.
	Add []string `yaml:"add,omitempty" json:"add,omitempty"`

	// Drop — capabilities to remove from the default set.
	// Use ["ALL"] to drop every capability before selectively adding back.
	Drop []string `yaml:"drop,omitempty" json:"drop,omitempty"`
}
