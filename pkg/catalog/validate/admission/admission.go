package admission

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// validateAdmissionRules checks all admission rules present in the catalog
// for correctness
func (c *checker) validateAdmissionRules() error {
	if err := c.validateMutationRules(); err != nil {
		return err
	}
	if err := c.validateValidationRules(); err != nil {
		return err
	}
	if err := c.validateValidationRuleLinks(); err != nil {
		return err
	}
	if err := c.validateAdmissionComparisons(); err != nil {
		return err
	}

	return nil
}

// validateAdmissionComparisons checks every validation.rules and
// mutation.rules entry, and their own when:/or: conditions: each compares one
// way, and a typeOf names a type.
func (c *checker) validateAdmissionComparisons() error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if crd.HasValidationRules() {
			for i, rule := range crd.EffectiveValidation().Rules {
				where := fmt.Sprintf("validation.rules[%d] (%s)", i, rule.Field)
				if err := rule.CheckComparison(); err != nil {
					return comparisonError(crd.Name, where, err)
				}
				if err := checkConditions(crd.Name, where, rule.When, rule.Or); err != nil {
					return err
				}
			}
		}
		if crd.HasMutationRules() {
			for i, rule := range crd.EffectiveMutation().Rules {
				where := fmt.Sprintf("mutation.rules[%d] (%s)", i, rule.Field)
				if err := checkConditions(crd.Name, where, rule.When, rule.Or); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkConditions(crdName, where string, when, or []types.Condition) error {
	for _, cond := range append(append([]types.Condition{}, when...), or...) {
		if err := cond.CheckComparison(); err != nil {
			return comparisonError(crdName, fmt.Sprintf("%s condition on %s", where, cond.Field), err)
		}
	}
	return nil
}

func comparisonError(crdName, where string, err error) error {
	return fmt.Errorf("%s CRD %q: %s %v", failureMark(), crdName, where, err)
}

// validateValidationRuleLinks rejects a validation.rules entry's link: value
// that either doesn't name any serve.fields / serve.labels and serve.annotations key on the
// CRD (typo, or the field was renamed/removed and this rule wasn't updated),
// or names an serve.fields key whose Field is already the redundant plain
// "spec.<name>" form — link: only earns its keep when Field isn't already a
// clean display name on its own. See ValidationRule.Link.
func (c *checker) validateValidationRuleLinks() error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasValidationRules() {
			continue
		}
		for _, rule := range crd.EffectiveValidation().Rules {
			if rule.Link == "" {
				continue
			}
			if err := checkLink(rule, crd); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkLink(rule types.ValidationRule, crd types.CRDEntry) error {
	if crd.ServeEnabled() {
		if _, ok := crd.Serve.Fields[rule.Link]; ok {
			if rule.Field == "spec."+rule.Link {
				return errRedundantLink(rule.Link, crd.Name)
			}
			return nil
		}
		if _, ok := crd.ServeLabels()[rule.Link]; ok {
			return nil
		}
		if _, ok := crd.ServeAnnotations()[rule.Link]; ok {
			return nil
		}
	}
	return errUnknownLink(rule.Link, crd.Name, rule.Field)
}

// validateMutationRules alks all declared and synthesized mutation rules
// and ensures that:
//
//   - rules.field is not empty
//   - Either default or override is declared - not both
//   - Adds warning if both are declared
func (c *checker) validateMutationRules() error {
	for name, crd := range c.Cat.EnabledCRDs() {
		// validate serve configuration
		if crd.ServeEnabled() {
			if err := c.validateServeMutationSynth(name); err != nil {
				return err
			}
		}

		if !crd.HasMutationRules() {
			continue
		}
		if crd.HasMutationRules() {
			for i, rule := range crd.EffectiveMutation().Rules {
				if rule.Field == "" {
					return fmt.Errorf("%s CRD: %s - mutation.rules[%d].field is required", failureMark(), crd.Name, i)
				}
				if !rule.IsValidChangeType() {
					return fmt.Errorf("%s CRD: %s - mutation.rules[%d] requires one of `default` or `override` to be defined", failureMark(), crd.Name, i)
				}
				if rule.HasDefaultAndOverride() {
					message := fmt.Sprintf("CRD: %s - mutation.rules[%d] has both default and override defined. default will be ignored.", crd.Name, i)
					crd.Warnings.AddWarning(message)
				}
			}
		}

		c.Cat.EnabledCRDs()[name] = crd
	}
	return nil
}

func (c *checker) validateServeMutationSynth(crdName string) error {
	for name, crd := range c.Cat.EnabledCRDs() {
		// serve.fields
		for name, cfg := range crd.ServeFields() {
			if cfg.HasDefault() {
				return fmt.Errorf("%s CRD: %s - 'default' is not allowed for serve.fields.%s since this is available from the CRD schema. You may use 'override' instead.",
					failureMark(), crdName, name)
			}
		}

		// serve.labels
		for name, cfg := range crd.ServeLabels() {
			if cfg.HasDefaultAndOverride() {
				message := fmt.Sprintf("CRD: %s - serve.labels.%s has both default and override defined. default will be ignored.", crdName, name)
				crd.Warnings.AddWarning(message)
			}
		}

		// serve.annotations
		for name, cfg := range crd.ServeAnnotations() {
			if cfg.HasDefaultAndOverride() {
				message := fmt.Sprintf("CRD: %s - serve.annotations.%s has both default and override defined. default will be ignored.", crdName, name)
				crd.Warnings.AddWarning(message)
			}
		}

		c.Cat.EnabledCRDs()[name] = crd
	}
	return nil
}

func (c *checker) validateValidationRules() error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasValidationRules() {
			continue
		}

		for i, rule := range crd.EffectiveValidation().Rules {
			if rule.Field == "" {
				return fmt.Errorf("%s CRD: %s - validation.rules[%d].field is required", failureMark(), crd.Name, i)
			}
			if rule.IsEmptyAssertions() {
				return fmt.Errorf("%s CRD: %s - validation.rules[%d] declares no comparison: use one of %s",
					failureMark(), crd.Name, i, strings.Join(types.ComparisonNames(), ", "))
			}
		}
	}
	return nil
}

func errUnknownLink(link, crd, field string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s link %q does not match any serve field
   CRD: %s
   field: %s

link: must name a key declared in serve.fields, serve.labels,
or serve.annotations for this CRD.
──────────────────────────────────────────────`, failureMark(), link, crd, field)
}

func errRedundantLink(link, crd string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s link %q is redundant
   CRD: %s

This rule's field is already "spec.%s" — already a clean display name on
its own. link: only matters when field is a template expression that isn't
itself a valid display name (e.g. wraps getLabel/getAnnotation, or a notes:
function). Consider removing link: here.
──────────────────────────────────────────────`, failureMark(), link, crd, link)
}
