package presets

import (
	"slices"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Builtin returns the built-in presets. Each call returns new slices, so a
// caller may change the result.
func Builtin() Registry {
	return Registry{
		Resources:           slices.Clone(builtinResources),
		Probes:              slices.Clone(builtinProbes),
		ContainerSecurities: slices.Clone(builtinContainerSecurities),
		PodSecurities:       slices.Clone(builtinPodSecurities),
		RollingUpdates:      slices.Clone(builtinRollingUpdates),
		HPAs:                slices.Clone(builtinHPAs),
		PDBs:                slices.Clone(builtinPDBs),
		NetworkPolicies:     slices.Clone(builtinNetworkPolicies),
		ResourceQuotas:      slices.Clone(builtinResourceQuotas),
	}
}

var builtinResources = []Resource{
	{Name: "tiny", Description: "Sidecars and small utilities.",
		Requests: map[string]string{"cpu": "25m", "memory": "64Mi"},
		Limits:   map[string]string{"cpu": "100m", "memory": "128Mi"}},
	{Name: "small", Description: "Light services.",
		Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
		Limits:   map[string]string{"cpu": "500m", "memory": "512Mi"}},
	{Name: "medium", Description: "Typical web services and APIs.",
		Requests: map[string]string{"cpu": "250m", "memory": "256Mi"},
		Limits:   map[string]string{"cpu": "1", "memory": "1Gi"}},
	{Name: "large", Description: "Busy services.",
		Requests: map[string]string{"cpu": "500m", "memory": "512Mi"},
		Limits:   map[string]string{"cpu": "2", "memory": "2Gi"}},
	{Name: "burst", Description: "Low steady use with room for spikes.",
		Requests: map[string]string{"cpu": "200m", "memory": "256Mi"},
		Limits:   map[string]string{"cpu": "2", "memory": "2Gi"}},
	{Name: "steady", Description: "Constant load, limits close to requests.",
		Requests: map[string]string{"cpu": "300m", "memory": "256Mi"},
		Limits:   map[string]string{"cpu": "600m", "memory": "512Mi"}},
	{Name: "compute-heavy", Description: "CPU-bound work.",
		Requests: map[string]string{"cpu": "1", "memory": "512Mi"},
		Limits:   map[string]string{"cpu": "2", "memory": "1Gi"}},
	{Name: "memory-heavy", Description: "Caches and in-memory data.",
		Requests: map[string]string{"cpu": "250m", "memory": "1Gi"},
		Limits:   map[string]string{"cpu": "500m", "memory": "2Gi"}},
}

var builtinProbes = []Probe{
	{Name: "fast", Description: "Quick detection, low tolerance: HTTP APIs that start fast.",
		InitialDelaySeconds: 5, PeriodSeconds: 10, FailureThreshold: 2, SuccessThreshold: 1, TimeoutSeconds: 5},
	{Name: "standard", Description: "Balanced timings for most web services; used when no preset is named.",
		InitialDelaySeconds: 15, PeriodSeconds: 20, FailureThreshold: 3, SuccessThreshold: 1, TimeoutSeconds: 10},
	{Name: "patient", Description: "Tolerant of slow operations: batch workers.",
		InitialDelaySeconds: 30, PeriodSeconds: 30, FailureThreshold: 5, SuccessThreshold: 1, TimeoutSeconds: 10},
	{Name: "slow-start", Description: "A 5-minute startup window: JVM apps, databases.",
		InitialDelaySeconds: 0, PeriodSeconds: 10, FailureThreshold: 30, SuccessThreshold: 1, TimeoutSeconds: 10},
}

var builtinContainerSecurities = []ContainerSecurity{
	{Name: "baseline", Description: "Prevents privilege escalation and drops NET_RAW.",
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &types.CapabilitiesConfig{Drop: []string{"NET_RAW"}}},
	{Name: "restricted", Description: "Drops all capabilities and requires non-root: the Kubernetes restricted Pod Security Standard.",
		AllowPrivilegeEscalation: ptr.To(false),
		RunAsNonRoot:             ptr.To(true),
		Capabilities:             &types.CapabilitiesConfig{Drop: []string{"ALL"}}},
	{Name: "hardened", Description: "Restricted, with a read-only root filesystem.",
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		Capabilities:             &types.CapabilitiesConfig{Drop: []string{"ALL"}}},
}

var builtinPodSecurities = []PodSecurity{
	{Name: "baseline", Description: "No user constraints.",
		RunAsNonRoot: ptr.To(false)},
	{Name: "restricted", Description: "Runs as a non-root user (1000).",
		RunAsNonRoot: ptr.To(true), RunAsUser: ptr.To[int64](1000)},
	{Name: "hardened", Description: "Runs as nobody (65534), user, group and filesystem group.",
		RunAsNonRoot: ptr.To(true), RunAsUser: ptr.To[int64](65534),
		RunAsGroup: ptr.To[int64](65534), FSGroup: ptr.To[int64](65534)},
}

var builtinRollingUpdates = []RollingUpdate{
	{Name: "safe", Description: "Adds one pod before removing one: no capacity drop during a rollout.",
		MaxSurge: "1", MaxUnavailable: "0"},
	{Name: "fast", Description: "Replaces pods in parallel with a brief capacity drop: the Kubernetes default.",
		MaxSurge: "25%", MaxUnavailable: "25%"},
	{Name: "blue-green", Description: "Brings up full duplicate capacity, then removes the old pods.",
		MaxSurge: "100%", MaxUnavailable: "0"},
}

var builtinHPAs = []HPA{
	{Name: "web", Description: "70% CPU, standard scale-up, 5-minute scale-down stabilization.",
		TargetCPUUtilizationPercentage: "70",
		Behavior: &types.HPABehavior{
			ScaleUp: &types.HPAScalingRules{
				StabilizationWindowSeconds: 0, SelectPolicy: "Max",
				Policies: []types.HPAScalingPolicy{
					{Type: "Percent", Value: 100, PeriodSeconds: 15},
					{Type: "Pods", Value: 4, PeriodSeconds: 15},
				},
			},
			ScaleDown: &types.HPAScalingRules{
				StabilizationWindowSeconds: 300, SelectPolicy: "Min",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 10, PeriodSeconds: 60}},
			},
		}},
	{Name: "api", Description: "60% CPU, fast scale-up, 10-minute scale-down stabilization.",
		TargetCPUUtilizationPercentage: "60",
		Behavior: &types.HPABehavior{
			ScaleUp: &types.HPAScalingRules{
				StabilizationWindowSeconds: 0, SelectPolicy: "Max",
				Policies: []types.HPAScalingPolicy{
					{Type: "Percent", Value: 100, PeriodSeconds: 15},
					{Type: "Pods", Value: 4, PeriodSeconds: 15},
				},
			},
			ScaleDown: &types.HPAScalingRules{
				StabilizationWindowSeconds: 600, SelectPolicy: "Min",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 5, PeriodSeconds: 60}},
			},
		}},
	{Name: "latency-sensitive", Description: "50% CPU, very fast scale-up, 15-minute scale-down stabilization.",
		TargetCPUUtilizationPercentage: "50",
		Behavior: &types.HPABehavior{
			ScaleUp: &types.HPAScalingRules{
				StabilizationWindowSeconds: 0, SelectPolicy: "Max",
				Policies: []types.HPAScalingPolicy{
					{Type: "Percent", Value: 200, PeriodSeconds: 15},
					{Type: "Pods", Value: 10, PeriodSeconds: 15},
				},
			},
			ScaleDown: &types.HPAScalingRules{
				StabilizationWindowSeconds: 900, SelectPolicy: "Min",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 5, PeriodSeconds: 120}},
			},
		}},
	{Name: "batch", Description: "80% CPU, moderate scale-up, 2-minute scale-down: jobs end fast.",
		TargetCPUUtilizationPercentage: "80",
		Behavior: &types.HPABehavior{
			ScaleUp: &types.HPAScalingRules{
				StabilizationWindowSeconds: 30, SelectPolicy: "Max",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 100, PeriodSeconds: 60}},
			},
			ScaleDown: &types.HPAScalingRules{
				StabilizationWindowSeconds: 120, SelectPolicy: "Min",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 50, PeriodSeconds: 60}},
			},
		}},
	{Name: "cost-optimized", Description: "80% CPU, slow scale-up, aggressive scale-down.",
		TargetCPUUtilizationPercentage: "80",
		Behavior: &types.HPABehavior{
			ScaleUp: &types.HPAScalingRules{
				StabilizationWindowSeconds: 180, SelectPolicy: "Min",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 25, PeriodSeconds: 60}},
			},
			ScaleDown: &types.HPAScalingRules{
				StabilizationWindowSeconds: 60, SelectPolicy: "Max",
				Policies: []types.HPAScalingPolicy{{Type: "Percent", Value: 50, PeriodSeconds: 60}},
			},
		}},
}

var builtinPDBs = []PDB{
	{Name: "zero-downtime", Description: "No pod may be voluntarily disrupted: stateful services, databases.",
		MinAvailable: "100%"},
	{Name: "rolling", Description: "One pod at a time: a safe default for production.",
		MaxUnavailable: "1"},
	{Name: "relaxed", Description: "Up to a quarter of pods at a time: stateless services.",
		MaxUnavailable: "25%"},
}

var builtinNetworkPolicies = []NetworkPolicy{
	{Name: "deny-all", Description: "Blocks all ingress and egress.",
		Ingress:     []types.NetworkPolicyIngressRule{},
		Egress:      []types.NetworkPolicyEgressRule{},
		PolicyTypes: []string{"Ingress", "Egress"}},
	{Name: "deny-all-ingress", Description: "Blocks all ingress.",
		Ingress:     []types.NetworkPolicyIngressRule{},
		PolicyTypes: []string{"Ingress"}},
	{Name: "deny-all-egress", Description: "Blocks all egress.",
		Egress:      []types.NetworkPolicyEgressRule{},
		PolicyTypes: []string{"Egress"}},
	{Name: "allow-same-namespace", Description: "Allows ingress from any pod in the same namespace.",
		Ingress:     []types.NetworkPolicyIngressRule{{From: []types.NetworkPolicyPeer{{PodSelector: map[string]string{}}}}},
		PolicyTypes: []string{"Ingress"}},
	{Name: "allow-dns-egress", Description: "Allows DNS egress on port 53, UDP and TCP.",
		Egress: []types.NetworkPolicyEgressRule{{Ports: []types.NetworkPolicyPort{
			{Protocol: "UDP", Port: "53"},
			{Protocol: "TCP", Port: "53"},
		}}},
		PolicyTypes: []string{"Egress"}},
}

var builtinResourceQuotas = []ResourceQuota{
	{Name: "small", Description: "10 pods, 2 CPU, 4Gi memory.", Hard: map[string]string{
		"pods": "10", "cpu": "2", "memory": "4Gi",
		"requests.cpu": "1", "requests.memory": "2Gi",
		"limits.cpu": "2", "limits.memory": "4Gi",
	}},
	{Name: "medium", Description: "20 pods, 4 CPU, 8Gi memory.", Hard: map[string]string{
		"pods": "20", "cpu": "4", "memory": "8Gi",
		"requests.cpu": "2", "requests.memory": "4Gi",
		"limits.cpu": "4", "limits.memory": "8Gi",
	}},
	{Name: "large", Description: "50 pods, 8 CPU, 16Gi memory.", Hard: map[string]string{
		"pods": "50", "cpu": "8", "memory": "16Gi",
		"requests.cpu": "4", "requests.memory": "8Gi",
		"limits.cpu": "8", "limits.memory": "16Gi",
	}},
	{Name: "xlarge", Description: "100 pods, 16 CPU, 32Gi memory.", Hard: map[string]string{
		"pods": "100", "cpu": "16", "memory": "32Gi",
		"requests.cpu": "8", "requests.memory": "16Gi",
		"limits.cpu": "16", "limits.memory": "32Gi",
	}},
}
