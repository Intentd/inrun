package profiles

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// HPAProfile is a named HPA scaling behavior preset.
//
//   - web              — 70% CPU, standard scale-up, 5-min scale-down stabilization.
//   - api              — 60% CPU, fast scale-up, 10-min scale-down stabilization.
//   - latency-sensitive — 50% CPU, very fast scale-up, 15-min scale-down stabilization.
//   - batch            — 80% CPU, moderate scale-up, 2-min scale-down (jobs end fast).
//   - cost-optimized   — 80% CPU, slow scale-up (3-min stabilization), aggressive scale-down.
type HPAProfile string

const (
	HPAWeb              HPAProfile = "web"
	HPAAPI              HPAProfile = "api"
	HPALatencySensitive HPAProfile = "latency-sensitive"
	HPABatch            HPAProfile = "batch"
	HPACostOptimized    HPAProfile = "cost-optimized"
)

// HPAProfileResult is returned by ApplyHPAProfile.
// It carries both the CPU utilization target and the fully-formed behavior block
// so callers can apply them to the HPA template in one step.
type HPAProfileResult struct {
	// CPUTarget is the suggested targetCPUUtilizationPercentage for this profile.
	// Applied only when the user has not declared an explicit value.
	CPUTarget int32

	// Behavior is the fully-expanded behavior block with Profile cleared.
	Behavior types.HPABehavior
}

// ApplyHPAProfile expands a named HPA profile into a CPUTarget and behavior block.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyHPAProfile(name string, reg *types.ProfileRegistry) (HPAProfileResult, error) {
	if reg != nil {
		if def, found := reg.LookupHPA(name); found {
			r := HPAProfileResult{}
			if def.TargetCPUUtilizationPercentage != "" {
				// static value only — template expressions resolved before this call
				var cpu int32
				if _, err := fmt.Sscanf(def.TargetCPUUtilizationPercentage, "%d", &cpu); err == nil {
					r.CPUTarget = cpu
				}
			}
			if def.Behavior != nil {
				r.Behavior = *def.Behavior
			}
			return r, nil
		}
	}
	switch HPAProfile(strings.ToLower(name)) {
	case HPAWeb:
		return HPAProfileResult{
			CPUTarget: 70,
			Behavior: types.HPABehavior{
				ScaleUp: &types.HPAScalingRules{
					StabilizationWindowSeconds: 0,
					SelectPolicy:               "Max",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 100, PeriodSeconds: 15},
						{Type: "Pods", Value: 4, PeriodSeconds: 15},
					},
				},
				ScaleDown: &types.HPAScalingRules{
					StabilizationWindowSeconds: 300,
					SelectPolicy:               "Min",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 10, PeriodSeconds: 60},
					},
				},
			},
		}, nil

	case HPAAPI:
		return HPAProfileResult{
			CPUTarget: 60,
			Behavior: types.HPABehavior{
				ScaleUp: &types.HPAScalingRules{
					StabilizationWindowSeconds: 0,
					SelectPolicy:               "Max",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 100, PeriodSeconds: 15},
						{Type: "Pods", Value: 4, PeriodSeconds: 15},
					},
				},
				ScaleDown: &types.HPAScalingRules{
					StabilizationWindowSeconds: 600,
					SelectPolicy:               "Min",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 5, PeriodSeconds: 60},
					},
				},
			},
		}, nil

	case HPALatencySensitive:
		return HPAProfileResult{
			CPUTarget: 50,
			Behavior: types.HPABehavior{
				ScaleUp: &types.HPAScalingRules{
					StabilizationWindowSeconds: 0,
					SelectPolicy:               "Max",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 200, PeriodSeconds: 15},
						{Type: "Pods", Value: 10, PeriodSeconds: 15},
					},
				},
				ScaleDown: &types.HPAScalingRules{
					StabilizationWindowSeconds: 900,
					SelectPolicy:               "Min",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 5, PeriodSeconds: 120},
					},
				},
			},
		}, nil

	case HPABatch:
		return HPAProfileResult{
			CPUTarget: 80,
			Behavior: types.HPABehavior{
				ScaleUp: &types.HPAScalingRules{
					StabilizationWindowSeconds: 30,
					SelectPolicy:               "Max",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 100, PeriodSeconds: 60},
					},
				},
				ScaleDown: &types.HPAScalingRules{
					StabilizationWindowSeconds: 120,
					SelectPolicy:               "Min",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 50, PeriodSeconds: 60},
					},
				},
			},
		}, nil

	case HPACostOptimized:
		return HPAProfileResult{
			CPUTarget: 80,
			Behavior: types.HPABehavior{
				ScaleUp: &types.HPAScalingRules{
					StabilizationWindowSeconds: 180,
					SelectPolicy:               "Min",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 25, PeriodSeconds: 60},
					},
				},
				ScaleDown: &types.HPAScalingRules{
					StabilizationWindowSeconds: 60,
					SelectPolicy:               "Max",
					Policies: []types.HPAScalingPolicy{
						{Type: "Percent", Value: 50, PeriodSeconds: 60},
					},
				},
			},
		}, nil

	default:
		return HPAProfileResult{}, fmt.Errorf(
			"unknown HPA profile: %q — allowed: web, api, latency-sensitive, batch, cost-optimized", name,
		)
	}
}

// IsValidHPAProfile reports whether name is a recognized HPA behavior profile.
func IsValidHPAProfile(name string) bool {
	switch HPAProfile(strings.ToLower(name)) {
	case HPAWeb, HPAAPI, HPALatencySensitive, HPABatch, HPACostOptimized:
		return true
	default:
		return false
	}
}
