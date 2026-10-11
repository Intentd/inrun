package types

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/inrundev/inrun/pkg/utils"
)

// serveFieldRef is one serve.fields, serve.labels or serve.annotations entry
// and the path its synthesized rules target.
type serveFieldRef struct {
	name  string // the serve key
	field string // its path: spec.<name>, or metadata.labels / metadata.annotations.<escaped key>
	link  string // the key, for ValidationRule.Link; empty for spec fields, whose path already reads well
	label string
	cfg   ServeFieldConfig
}

// labelPath and annotationPath are the paths of a label or annotation key;
// dots in the key are escaped (see utils.SplitField).
func labelPath(key string) string      { return "metadata.labels." + utils.EscapePathKey(key) }
func annotationPath(key string) string { return "metadata.annotations." + utils.EscapePathKey(key) }

// allServeFieldRefs returns one serveFieldRef per serve field, label and
// annotation, sorted like the console form: by order (0 last), then name. The
// field shown first is also the one whose violation is reported first.
func (c *CRDEntry) allServeFieldRefs() []serveFieldRef {
	if c == nil || c.Serve == nil {
		return nil
	}

	var refs []serveFieldRef
	for name, cfg := range c.Serve.Fields {
		// "spec." + name is already a clean display name on its own —
		// no link needed, and setting one would trip the "redundant link"
		// validation error.
		refs = append(refs, serveFieldRef{name: name, field: "spec." + name, label: serveFieldLabel(cfg, name), cfg: cfg})
	}
	for name, cfg := range c.ServeLabels() {
		refs = append(refs, serveFieldRef{name: name, field: labelPath(name), link: name, label: serveFieldLabel(cfg, name), cfg: cfg})
	}
	for name, cfg := range c.ServeAnnotations() {
		refs = append(refs, serveFieldRef{name: name, field: annotationPath(name), link: name, label: serveFieldLabel(cfg, name), cfg: cfg})
	}

	sort.Slice(refs, func(i, j int) bool {
		oi, oj := refs[i].cfg.Order, refs[j].cfg.Order
		if oi == 0 && oj != 0 {
			return false
		}
		if oi != 0 && oj == 0 {
			return true
		}
		if oi != oj {
			return oi < oj
		}
		return refs[i].name < refs[j].name
	})
	return refs
}

// DuplicateServeFieldOrders returns each non-zero order shared by more than one
// serve field, label or annotation, with the field names sorted. Order decides
// which violation is reported first, so a shared value is an error.
func (c *CRDEntry) DuplicateServeFieldOrders() map[int][]string {
	if c == nil || c.Serve == nil {
		return nil
	}
	byOrder := map[int][]string{}
	for _, ref := range c.allServeFieldRefs() {
		if ref.cfg.Order == 0 {
			continue
		}
		byOrder[ref.cfg.Order] = append(byOrder[ref.cfg.Order], ref.name)
	}
	var dups map[int][]string
	for order, names := range byOrder {
		if len(names) > 1 {
			if dups == nil {
				dups = map[int][]string{}
			}
			slices.Sort(names)
			dups[order] = names
		}
	}
	return dups
}

func serveFieldLabel(cfg ServeFieldConfig, name string) string {
	if cfg.Label != "" {
		return cfg.Label
	}
	return name
}

// RequiredServeFieldRules returns an exists rule for every serve field, label
// or annotation marked required, so every gateway client is checked, not just
// the console form. Each rule keeps the field's when/or.
func (c *CRDEntry) RequiredServeFieldRules() []ValidationRule {
	var rules []ValidationRule
	for _, ref := range c.allServeFieldRefs() {
		if !ref.cfg.Required {
			continue
		}
		rules = append(rules, ValidationRule{
			Field:    ref.field,
			Link:     ref.link,
			Operator: ConditionExists,
			Message:  ref.label + " is required",
			Action:   ValidationActionDeny,
			When:     ref.cfg.When,
			Or:       ref.cfg.Or,
		})
	}
	return rules
}

// EnumServeFieldRules returns an `in` rule for every enum serve field, label or
// annotation. It checks a value only when one is set; whether one must be set
// is RequiredServeFieldRules' job.
func (c *CRDEntry) EnumServeFieldRules() []ValidationRule {
	var rules []ValidationRule
	for _, ref := range c.allServeFieldRefs() {
		cfgType := strings.ToLower(ref.cfg.Type)
		if cfgType != "enum" || len(ref.cfg.Enum) == 0 {
			continue
		}
		when := append([]Condition{{Field: ref.field, Operator: ConditionExists}}, ref.cfg.When...)
		rules = append(rules, ValidationRule{
			Field:    ref.field,
			Link:     ref.link,
			Operator: ConditionIn,
			Value:    strings.Join(ref.cfg.Enum, ","),
			Message:  ref.label + " must be one of: " + strings.Join(ref.cfg.Enum, ", "),
			Action:   ValidationActionDeny,
			When:     when,
			Or:       ref.cfg.Or,
		})
	}
	return rules
}

// ServeFieldMutationRules returns a mutation rule for every serve field, label
// or annotation with a default or override. Spec fields honour only override,
// since the CRD schema carries their defaults.
func (c *CRDEntry) ServeFieldMutationRules() []MutationRule {
	if c == nil || c.Serve == nil {
		return nil
	}

	var rules []MutationRule
	// serve.fields
	for name, cfg := range c.ServeFields() {
		if !cfg.HasOverride() {
			continue
		}
		field := cfg.Path
		if field == "" {
			field = "spec." + name
		}

		// resolve default/override
		if cfg.Override != nil {
			cfg.Default = nil
		}

		rules = append(rules, MutationRule{
			Field:     field,
			Default:   cfg.Default,
			Override:  cfg.Override,
			ValueType: cfg.Type,
			When:      cfg.When,
			Or:        cfg.Or,
		})
	}

	// serve.labels
	for name, cfg := range c.ServeLabels() {
		if !cfg.HasDefault() && !cfg.HasOverride() {
			continue
		}

		// resolve default/override
		if cfg.Override != nil {
			cfg.Default = nil
		}
		rules = append(rules, MutationRule{
			Field:     labelPath(name),
			Default:   cfg.Default,
			Override:  cfg.Override,
			ValueType: cfg.Type,
			When:      cfg.When,
			Or:        cfg.Or,
		})
	}

	// serve.annotations
	for name, cfg := range c.ServeAnnotations() {
		if !cfg.HasDefault() && !cfg.HasOverride() {
			continue
		}

		// resolve default/override
		if cfg.Override != nil {
			cfg.Default = nil
		}
		rules = append(rules, MutationRule{
			Field:     annotationPath(name),
			Default:   cfg.Default,
			Override:  cfg.Override,
			ValueType: cfg.Type,
			When:      cfg.When,
			Or:        cfg.Or,
		})
	}

	return rules
}

type SynthDedup struct {
	HasValidation   bool
	HasMutation     bool
	ValidationRules []ValidationRule
	MutationRules   []MutationRule
}

// DeduplicateSynthesizedServeRules removes synthesized rules that are
// already present — either because this is a bundle.yaml that baked them in at
// generate time (inrun generate bundle calls SerializeExpanded, which serializes
// the post-synthesis state), or because a prior load pass already ran synthesis.
// Synthesis is deterministic, so an exact match is always a safe duplicate to drop.
func (c *CRDEntry) DeduplicateSynthesizedServeRules(synth SynthDedup) {
	if c == nil {
		return
	}

	switch {
	case synth.HasValidation:
		if c.Admission == nil {
			c.Admission = &AdmissionConfig{}
		}
		if c.Admission.Validation == nil {
			c.Admission.Validation = &ValidationConfig{}
		}
		var toAddVal []ValidationRule
		for _, s := range synth.ValidationRules {
			dup := false
			for _, existing := range c.Admission.Validation.Rules {
				if reflect.DeepEqual(s, existing) {
					dup = true
					break
				}
			}
			if !dup {
				toAddVal = append(toAddVal, s)
			}
		}
		if len(toAddVal) > 0 {
			c.Admission.Validation.Rules = append(toAddVal, c.Admission.Validation.Rules...)
		}
	case synth.HasMutation:
		if c.Admission == nil {
			c.Admission = &AdmissionConfig{}
		}
		if c.Admission.Mutation == nil {
			c.Admission.Mutation = &MutationConfig{}
		}
		var toAddMut []MutationRule
		for _, s := range synth.MutationRules {
			dup := false
			for _, existing := range c.Admission.Mutation.Rules {
				if reflect.DeepEqual(s, existing) {
					dup = true
					break
				}
			}
			if !dup {
				toAddMut = append(toAddMut, s)
			}
		}
		if len(toAddMut) > 0 {
			c.Admission.Mutation.Rules = append(toAddMut, c.Admission.Mutation.Rules...)
		}

	}
}
