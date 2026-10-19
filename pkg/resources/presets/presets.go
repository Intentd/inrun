package presets

import (
	"strconv"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Resource is a named set of CPU and memory requests and limits.
type Resource struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Requests    map[string]string `yaml:"requests,omitempty" json:"requests,omitempty"`
	Limits      map[string]string `yaml:"limits,omitempty" json:"limits,omitempty"`
}

// Requirements returns the preset as a container's resource requirements.
func (p Resource) Requirements() *types.ResourceRequirements {
	return &types.ResourceRequirements{Requests: p.Requests, Limits: p.Limits}
}

// Probe is a named set of probe timings.
type Probe struct {
	Name                string `yaml:"name" json:"name"`
	Description         string `yaml:"description,omitempty" json:"description,omitempty"`
	InitialDelaySeconds int32  `yaml:"initialDelaySeconds,omitempty" json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32  `yaml:"periodSeconds,omitempty" json:"periodSeconds,omitempty"`
	FailureThreshold    int32  `yaml:"failureThreshold,omitempty" json:"failureThreshold,omitempty"`
	SuccessThreshold    int32  `yaml:"successThreshold,omitempty" json:"successThreshold,omitempty"`
	TimeoutSeconds      int32  `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
}

// ContainerSecurity is a named container security context.
type ContainerSecurity struct {
	Name                     string                    `yaml:"name" json:"name"`
	Description              string                    `yaml:"description,omitempty" json:"description,omitempty"`
	AllowPrivilegeEscalation *bool                     `yaml:"allowPrivilegeEscalation,omitempty" json:"allowPrivilegeEscalation,omitempty"`
	ReadOnlyRootFilesystem   *bool                     `yaml:"readOnlyRootFilesystem,omitempty" json:"readOnlyRootFilesystem,omitempty"`
	RunAsNonRoot             *bool                     `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`
	RunAsUser                *int64                    `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`
	RunAsGroup               *int64                    `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`
	Capabilities             *types.CapabilitiesConfig `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// Context returns the preset as a container security context.
func (p ContainerSecurity) Context() *types.ContainerSecurityContext {
	return &types.ContainerSecurityContext{
		AllowPrivilegeEscalation: p.AllowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   p.ReadOnlyRootFilesystem,
		RunAsNonRoot:             p.RunAsNonRoot,
		RunAsUser:                p.RunAsUser,
		RunAsGroup:               p.RunAsGroup,
		Capabilities:             p.Capabilities,
	}
}

// PodSecurity is a named pod security context.
type PodSecurity struct {
	Name         string `yaml:"name" json:"name"`
	Description  string `yaml:"description,omitempty" json:"description,omitempty"`
	RunAsNonRoot *bool  `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`
	RunAsUser    *int64 `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`
	RunAsGroup   *int64 `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`
	FSGroup      *int64 `yaml:"fsGroup,omitempty" json:"fsGroup,omitempty"`
}

// Context returns the preset as a pod security context.
func (p PodSecurity) Context() *types.PodSecurityContext {
	return &types.PodSecurityContext{
		RunAsNonRoot: p.RunAsNonRoot,
		RunAsUser:    p.RunAsUser,
		RunAsGroup:   p.RunAsGroup,
		FSGroup:      p.FSGroup,
	}
}

// RollingUpdate is a named Deployment rolling update strategy. Each value is
// a count ("1") or a percentage ("25%").
type RollingUpdate struct {
	Name           string `yaml:"name" json:"name"`
	Description    string `yaml:"description,omitempty" json:"description,omitempty"`
	MaxSurge       string `yaml:"maxSurge,omitempty" json:"maxSurge,omitempty"`
	MaxUnavailable string `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`
}

// Behavior returns the preset as a rolling update behavior.
func (p RollingUpdate) Behavior() *types.RollingUpdateBehavior {
	return &types.RollingUpdateBehavior{MaxSurge: p.MaxSurge, MaxUnavailable: p.MaxUnavailable}
}

// HPA is a named HorizontalPodAutoscaler setup: a CPU target and scaling
// behavior. The string fields may hold template expressions in a Blueprint,
// resolved before use.
type HPA struct {
	Name                           string             `yaml:"name" json:"name"`
	Description                    string             `yaml:"description,omitempty" json:"description,omitempty"`
	MinReplicas                    string             `yaml:"minReplicas,omitempty" json:"minReplicas,omitempty"`
	MaxReplicas                    string             `yaml:"maxReplicas,omitempty" json:"maxReplicas,omitempty"`
	TargetCPUUtilizationPercentage string             `yaml:"targetCPUUtilizationPercentage,omitempty" json:"targetCPUUtilizationPercentage,omitempty"`
	Behavior                       *types.HPABehavior `yaml:"behavior,omitempty" json:"behavior,omitempty"`
}

// CPUTarget returns the CPU utilization target as a number, or 0 when it is
// unset or not a whole number.
func (p HPA) CPUTarget() int32 {
	n, err := strconv.ParseInt(p.TargetCPUUtilizationPercentage, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

// PDB is a named PodDisruptionBudget limit: MinAvailable or MaxUnavailable,
// each a count or a percentage.
type PDB struct {
	Name           string `yaml:"name" json:"name"`
	Description    string `yaml:"description,omitempty" json:"description,omitempty"`
	MinAvailable   string `yaml:"minAvailable,omitempty" json:"minAvailable,omitempty"`
	MaxUnavailable string `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`
}

// NetworkPolicy is a named set of NetworkPolicy rules.
type NetworkPolicy struct {
	Name        string                           `yaml:"name" json:"name"`
	Description string                           `yaml:"description,omitempty" json:"description,omitempty"`
	PodSelector map[string]interface{}           `yaml:"podSelector,omitempty" json:"podSelector,omitempty"`
	Ingress     []types.NetworkPolicyIngressRule `yaml:"ingress,omitempty" json:"ingress,omitempty"`
	Egress      []types.NetworkPolicyEgressRule  `yaml:"egress,omitempty" json:"egress,omitempty"`
	PolicyTypes []string                         `yaml:"policyTypes,omitempty" json:"policyTypes,omitempty"`
}

// ResourceQuota is a named set of a namespace's hard limits.
type ResourceQuota struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Hard        map[string]string `yaml:"hard" json:"hard"`
}

// LimitRange is a named set of LimitRange items. There are no built-in ones.
type LimitRange struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Limits      []types.LimitRangeItem `yaml:"limits" json:"limits"`
}

func (p Resource) presetName() string          { return p.Name }
func (p Probe) presetName() string             { return p.Name }
func (p ContainerSecurity) presetName() string { return p.Name }
func (p PodSecurity) presetName() string       { return p.Name }
func (p RollingUpdate) presetName() string     { return p.Name }
func (p HPA) presetName() string               { return p.Name }
func (p PDB) presetName() string               { return p.Name }
func (p NetworkPolicy) presetName() string     { return p.Name }
func (p ResourceQuota) presetName() string     { return p.Name }
func (p LimitRange) presetName() string        { return p.Name }
