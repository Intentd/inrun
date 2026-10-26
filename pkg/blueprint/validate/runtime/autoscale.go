// Autoscaler Preset Validation
//
// Presets are computed presets. When a preset is declared, it must be the
// *only* autoscale input. Presets expand into a complete AutoscaleSpec during
// blueprint enrichment, so mixing them with manual fields creates ambiguity.
//
// Validation enforces:
//
// 1. Preset-only usage:
//    `autoscale.preset` cannot appear alongside interval, cooldown,
//    conditions, or do overrides.
//
// 2. Known preset names:
//    Allowed: burst, steady, batch, latency-sensitive, cost-optimized.
//
// 3. Valid baseline (checked before expansion):
//    Baseline workers and queueDepth must be > 0.
//
// 4. Expansion safety:
//    The generated AutoscaleSpec must contain a valid interval, cooldown,
//    non-empty conditions, and a non-empty do block.
//
// 5. No cross-operator usage:
//    Presets are local-metric presets and cannot be combined with cross-
//    operator autoscale conditions.
//
// 6. No partial overrides:
//    Users cannot override parts of a preset (e.g., adding a do block).
//
// Presets are atomic and predictable. If a preset is set, it fully defines
// autoscale behavior for that operator.

package runtime

import (
	"fmt"

	"github.com/intentd/inrun/pkg/presets"
	"github.com/intentd/inrun/pkg/types"
)

// validateAutoscalePreset ensures that autoscale.preset is used correctly
// and expands cleanly. applyDefaults writes the expansion, so the runtime only
// ever sees a fully-formed spec (never a bare preset name).
func (c *checker) validateAutoscalePreset() error {
	for _, crd := range c.Blueprint.EnabledCRDs() {
		spec := crd.Box().EffectiveAutoscale()

		// No autoscale block → nothing to validate
		if spec == nil {
			continue
		}

		// No preset declared → nothing to validate
		if !crd.HasAutoscalePreset() {
			continue
		}

		preset := crd.AutoScalePreset()

		// Rule 1: preset cannot be combined with manual autoscale fields
		if !autoscaleIsPresetOnly(spec) {
			return fmt.Errorf(
				"%s autoscale.preset %q cannot be combined with manual autoscale fields; "+
					"remove interval/cooldown/conditions/do when using a preset",
				failureMark(), preset,
			)
		}

		// Rule 2: preset must be recognized
		if !presets.IsValidAutoscalePreset(preset) {
			return fmt.Errorf("%s unknown autoscale preset: %q", failureMark(), preset)
		}

		if _, err := ExpandAutoscalePreset(&crd); err != nil {
			return fmt.Errorf("%s autoscale.preset %q expansion failed: %w", failureMark(), preset, err)
		}
	}
	return nil
}

// ExpandAutoscalePreset expands crd's autoscale.preset into a fully-formed
// AutoscaleSpec, using the CRD's declared workers, queue depth and resync as
// the baseline.
func ExpandAutoscalePreset(crd *types.CRDEntry) (*types.AutoscaleSpec, error) {
	return presets.ApplyAutoscalerPreset(crd.AutoScalePreset(), types.AutoscaleBaseline{
		Workers:  crd.SetWorkers(0),
		MaxDepth: crd.SetQueueDepth(0),
		Resync:   crd.SetResync(0),
	})
}

// autoscaleIsPresetOnly returns true when the user has declared ONLY a preset
// and no manual autoscale fields.
func autoscaleIsPresetOnly(spec *types.AutoscaleSpec) bool {
	if spec == nil {
		return true
	}

	// Any manual field invalidates preset usage
	if spec.HasCooldownDuration() || spec.HasIntervalDuration() ||
		spec.HasWhenConditions() || spec.HasOrConditions() ||
		spec.HasDoWorkers() || spec.HasDoQueueDepth() ||
		spec.HasDoResync() {
		return false
	}

	return true
}
