package blueprint

import "github.com/intentd/inrun/pkg/types"

// applyServeAdmissionSynthesis synthesizes implicit admission rules from serve field declarations.
// Fields marked required: true get an exists validation rule; type: enum fields get an in rule.
// Fields with default: or override: get mutation rules via ServeFieldMutationRules.
// Runs last, after all explicit rules (inline, include, module) have settled. Synthesized
// validation rules are prepended so a missing field surfaces as the headline denial reason.
func (c *Blueprint) applyServeAdmissionSynthesis() {
	for name, entry := range c.enabledCRDs {
		synthesizedVal := append(entry.RequiredServeFieldRules(), entry.EnumServeFieldRules()...)
		synthesizedMut := entry.ServeFieldMutationRules()

		hasSynthVal := len(synthesizedVal) > 0
		hasSynthMut := len(synthesizedMut) > 0

		if !hasSynthVal && !hasSynthMut {
			continue
		}

		if hasSynthVal {
			if entry.Admission == nil {
				entry.Admission = &types.AdmissionConfig{}
			}
			if entry.Admission.Validation == nil {
				entry.Admission.Validation = &types.ValidationConfig{}
			}
		}
		if hasSynthMut {
			if entry.Admission == nil {
				entry.Admission = &types.AdmissionConfig{}
			}
			if entry.Admission.Mutation == nil {
				entry.Admission.Mutation = &types.MutationConfig{}
			}
		}
		entry.DeduplicateSynthesizedServeRules(types.SynthDedup{
			HasValidation:   hasSynthVal,
			HasMutation:     hasSynthMut,
			ValidationRules: synthesizedVal,
			MutationRules:   synthesizedMut,
		})
		c.enabledCRDs[name] = entry
	}
}
