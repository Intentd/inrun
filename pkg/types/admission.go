// pkg/types/admission.go
package types

// ── Validation and Mutation ────────────────────────────────────────────────
//
// These declarations control policy enforcement at two points:
//
//   1. Admission time  — when ENABLE_ADMISSION_WEBHOOK=true, the API server calls
//      Inrun's /validate and /mutate endpoints synchronously during
//      kubectl apply. The CR is rejected or mutated before storage.
//
//   2. Reconcile time  — the same rules run inside the reconcile loop for
//      CRs that existed before the webhook was enabled, and as drift
//      correction for any rule violations that appear after admission.
//
// Both enforcement points read from the same Catalog declaration. There
// is no duplication — declare once, enforce everywhere.
//
// Example:
//
//   operatorBox:
//     validation:
//       - field: spec.image
//         prefix: "myorg/"
//         message: "image must be from myorg registry"
//         action: deny   # synchronous rejection at admission + halt at reconcile
//
//     mutation:
//       - field: spec.replicas
//         default: "2"   # applied at admission time, and on first reconcile

// ── ValidationAction ──────────────────────────────────────────────────────

// ValidationAction declares what happens when a validation rule fails.
//
//	deny  (default) — at admission: synchronous rejection, kubectl sees the error
//	                  at reconcile: reconciliation halted until CR is corrected
//
//	warn            — at admission: Kubernetes Warning header, CR is still stored
//	                  at reconcile: recorded as active warning on health API
//
// Default is deny when action is omitted. Warn must be declared explicitly.
// This is intentional — new rules block by default.
type ValidationAction string

const (
	ValidationActionDeny ValidationAction = "deny"
	ValidationActionWarn ValidationAction = "warn"
)

// EffectiveAction returns the effective action, defaulting to deny.
func EffectiveAction(a ValidationAction) ValidationAction {
	if a == "" {
		return ValidationActionDeny
	}
	return a
}

func (a ValidationAction) IsDeny() bool {
	return EffectiveAction(a) == ValidationActionDeny
}

func (a ValidationAction) IsWarn() bool {
	return EffectiveAction(a) == ValidationActionWarn
}

// Admission holds validation and mutation configuration for a CRD.
type Admission struct {
	Validation *ValidationConfig
	Mutation   *MutationConfig
}

// Returns true when either validation or mutation rules are declared.
func (a *Admission) HasValidationOrMutationRules() bool {
	return a.HasValidationRules() || a.HasMutationRules()
}

// Separate helpers for hasMutationRules and hasValidationRules
func (a *Admission) HasMutationRules() bool {
	if a.Mutation == nil {
		return false
	}
	return len(a.Mutation.Rules) > 0
}

func (a *Admission) HasValidationRules() bool {
	if a.Validation == nil {
		return false
	}
	return len(a.Validation.Rules) > 0
}

func (a *Admission) HasMutationExternal() bool {
	if a.Mutation == nil {
		return false
	}
	return len(a.Mutation.External) > 0
}

func (a *Admission) HasValidationExternal() bool {
	if a.Validation == nil {
		return false
	}
	return len(a.Validation.External) > 0
}

// ── ValidationRule ────────────────────────────────────────────────────────

// ValidationRule declares one constraint on a CR field.
//
// Shorthands (prefix, suffix, equals, etc.) exist so common rules read
// without boilerplate. Exactly one shorthand or an explicit operator+value
// pair should be set per rule.
//
// Example — deny:
//
//   - field: spec.image
//     prefix: "myorg/"
//     message: "image must be from myorg registry"
//     action: deny
//
// Example — warn, advisory only:
//
//   - field: metadata.labels.team
//     exists: true
//     message: "all resources should declare a team owner"
//     action: warn
//
// Example — numeric bound:
//
//   - field: spec.replicas
//     max: "10"
//     message: "replicas cannot exceed 10"
type ValidationRule struct {
	// Field — dot-notation path to the CR field being validated.
	// e.g. "spec.image", "metadata.labels.tier", "spec.db.engine"
	Field string `yaml:"field" json:"field" validate:"required"`

	// Message — shown in the Kubernetes event, webhook response, and operator
	// log when this rule is violated. Make it actionable.
	Message string `yaml:"message" json:"message" validate:"required"`

	// Action — what happens when this rule fails.
	// deny (default): block at admission, halt at reconcile.
	// warn: warning at admission, advisory at reconcile.
	Action ValidationAction `yaml:"action,omitempty" json:"action,omitempty"`

	// Comparisons: set exactly one. Same names and meaning as Condition's;
	// min and max are inclusive, greaterThan and lessThan strict.
	Equals      string `yaml:"equals,omitempty" json:"equals,omitempty"`
	NotEquals   string `yaml:"notEquals,omitempty" json:"notEquals,omitempty"`
	Prefix      string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	NotPrefix   string `yaml:"notPrefix,omitempty" json:"notPrefix,omitempty"`
	Suffix      string `yaml:"suffix,omitempty" json:"suffix,omitempty"`
	NotSuffix   string `yaml:"notSuffix,omitempty" json:"notSuffix,omitempty"`
	Contains    string `yaml:"contains,omitempty" json:"contains,omitempty"`
	Min         string `yaml:"min,omitempty" json:"min,omitempty"` // numeric, inclusive
	Max         string `yaml:"max,omitempty" json:"max,omitempty"` // numeric, inclusive
	GreaterThan string `yaml:"greaterThan,omitempty" json:"greaterThan,omitempty"`
	LessThan    string `yaml:"lessThan,omitempty" json:"lessThan,omitempty"`

	NotContains string `yaml:"notContains,omitempty" json:"notContains,omitempty"`

	// Regex is an RE2 pattern.
	Regex string `yaml:"regex,omitempty" json:"regex,omitempty"`

	// Between and NotBetween take an inclusive "min,max" range.
	Between string `yaml:"between,omitempty" json:"between,omitempty"`

	NotBetween string `yaml:"notBetween,omitempty" json:"notBetween,omitempty"`

	// In and NotIn take a comma-separated list.
	In string `yaml:"in,omitempty" json:"in,omitempty"`

	NotIn string `yaml:"notIn,omitempty" json:"notIn,omitempty"`

	// TypeOf passes when the field's value has this type: map, slice,
	// string, number, bool or null.
	TypeOf FieldType `yaml:"typeOf,omitempty" json:"typeOf,omitempty"`

	// Exists and NotExists require the field to be present (non-empty) or
	// absent.
	Exists    *bool `yaml:"exists,omitempty" json:"exists,omitempty"`
	NotExists *bool `yaml:"notExists,omitempty" json:"notExists,omitempty"`

	// Unique, when true, requires that no other instance of the CRD has the
	// same value at Field.
	Unique *bool `yaml:"unique,omitempty" json:"unique,omitempty"`

	ValueType string `yaml:"valueType,omitempty"` // "string", "int" or "integer", "float" or "number", "bool" or "boolean"

	// When — all conditions must pass for this rule to be evaluated (AND).
	// Empty means unconditional.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or — at least one condition must pass for this rule to be evaluated (OR).
	// When both When and Or are declared, both blocks must pass.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// Fires controls at which lifecycle points this rule is evaluated.
	// Absent: fires at both admission and reconcile time.
	// fires.reconcile: false — admission only; reconciler skips this rule.
	// Example: use for rules that read .intent.* (raw intent), which is only present
	// at the admission boundary, not during reconcile.
	Fires *FiresConfig `yaml:"fires,omitempty" json:"fires,omitempty"`

	// Link is the serve.fields, serve.labels or serve.annotations key this
	// rule is about. Violations report it instead of Field, so a form (the
	// Console, any Gateway API client) can highlight the input it rendered.
	// Set it when Field is an expression rather than a plain path, for
	// example a note call. A literal key, never a template; it must be
	// declared on the CRD, and linking a plain "spec.<name>" Field is
	// rejected as redundant. Rules synthesized from required or enum serve
	// entries set it themselves.
	//
	//	validation:
	//	  rules:
	//	    - field: '{{ isDNS1123Subdomain team }}'
	//	      link: team
	//	      equals: "true"
	//	      message: "team must be a valid DNS subdomain"
	//	      action: deny
	Link string `yaml:"link,omitempty" json:"link,omitempty"`
}

// IsEmptyAssertions reports whether the rule declares no comparison (it then
// means exists).
func (r ValidationRule) IsEmptyAssertions() bool {
	return len(r.Declared()) == 0
}

// ValidationRuleFor returns the first matching mutation rule for a given field
func ValidationRuleFor(rules []ValidationRule, field string) ValidationRule {
	if rules == nil && field != "" {
		return ValidationRule{}
	}
	for _, rule := range rules {
		if rule.Field == field {
			return rule
		}
	}
	return ValidationRule{}
}

// ValidationRulesFor returns the all matching mutation rules for a given field
func ValidationRulesFor(rules []ValidationRule, field string) []ValidationRule {
	if rules == nil && field != "" {
		return []ValidationRule{}
	}

	result := []ValidationRule{}
	for _, rule := range rules {
		if rule.Field == field {
			result = append(result, rule)
		}
	}
	return result
}

// ValidationConfig holds all validation rules for a CRD.
type ValidationConfig struct {
	// Include is a path (relative to the catalog file) to a YAML file whose
	// top-level value is a list of ValidationRule entries. Expanded at load
	// time — included rules come first, inline rules append after.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`

	// External declares HTTP calls made before validation rules are evaluated.
	// Results are available in rule field:, equals:, and message: expressions.
	// At admission time: all entries fire.
	// At reconcile time: only entries with fires.reconcile != false fire.
	External []ExternalCallSpec `yaml:"external,omitempty" json:"external,omitempty"`

	// Rules — evaluated in declaration order.
	// All rules are evaluated before returning — all violations are reported
	// together so the user can fix everything in one cycle.
	Rules []ValidationRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// AdmissionExternal returns all external calls — every entry fires at admission time.
func (c *ValidationConfig) AdmissionExternal() []ExternalCallSpec {
	if c == nil {
		return nil
	}
	return c.External
}

// HasExternalCall reports whether there is any external call in the config
func (c *ValidationConfig) HasExternalCall() bool {
	if c == nil {
		return false
	}
	return len(c.External) > 0
}

// ReconcileExternal returns only calls that fire at reconcile time.
// Entries with fires.reconcile: false are excluded.
func (c *ValidationConfig) ReconcileExternal() []ExternalCallSpec {
	if c == nil {
		return nil
	}
	var out []ExternalCallSpec
	for _, e := range c.External {
		if e.Fires.FiresAtReconcile() {
			out = append(out, e)
		}
	}
	return out
}

// HasDenyRules reports whether any rules use action: deny (or the default).
// Used to decide whether to register a ValidatingWebhookConfiguration.
func (c *ValidationConfig) HasDenyRules() bool {
	if c == nil {
		return false
	}
	for _, r := range c.Rules {
		if r.Action.IsDeny() || r.Action == "" {
			return true
		}
	}
	return false
}

// HasWarnRules reports whether any rules use action: warn.
func (c *ValidationConfig) HasWarnRules() bool {
	if c == nil {
		return false
	}
	for _, r := range c.Rules {
		if r.Action.IsWarn() {
			return true
		}
	}
	return false
}

// HasAnyRules reports whether any validation rule is configured
func (c *ValidationConfig) HasAnyRules() bool {
	if c == nil {
		return false
	}
	return len(c.Rules) > 0
}

// HasUniqueRule reports whether any rule sets unique: true.
func (c *ValidationConfig) HasUniqueRule() bool {
	if c == nil {
		return false
	}
	for _, r := range c.Rules {
		if r.Unique != nil && *r.Unique {
			return true
		}
	}
	return false
}

// References reports whether any rule's field reads the data root (for
// example "health" or "metrics"), directly or through a note. Used by
// validators and also the webhook to fetch data only when a rule needs it.
func (c *ValidationConfig) References(nr NoteRegistry, root string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Rules {
		if nr.References(r.Field, root) {
			return true
		}
	}
	return false
}

// ── MutationRule ──────────────────────────────────────────────────────────

// MutationRule declares one field mutation.
//
// Two mutation types:
//
//	default  — set the field only if it is absent or empty.
//	           At admission time: applied before validation runs.
//	           At reconcile time: applied on first reconcile cycle.
//
//	override — always set the field, regardless of current value.
//	           Use with caution — overwrites user-provided values.
//
// Both types support Go template expressions resolved against the CR object:
//
//	default: "{{ .metadata.name }}-default"
//	override: "myorg/{{ .metadata.name }}:latest"
//
// Example:
//
//	mutation:
//	  - field: spec.replicas
//	    default: "2"
//	  - field: spec.logLevel
//	    default: "info"
//	  - field: spec.image
//	    override: "myorg/{{ .metadata.name }}:latest"
type MutationRule struct {
	// Field — dot-notation path to the CR field to mutate.
	Field string `yaml:"field" json:"field" validate:"required"`

	// Default — set only if the field is absent or empty.
	// Supports template expressions.
	Default interface{} `yaml:"default,omitempty"` // accepts int, bool, string from YAML

	// Override — always set, regardless of current value.
	// Supports template expressions.
	Override interface{} `yaml:"override,omitempty"` // accepts int, bool, string from YAML

	// Value type
	ValueType string `yaml:"valueType,omitempty"` // "string", "int" or "integer", "float" or "number", "bool" or "boolean"

	// When — all conditions must pass for this rule to be applied (AND).
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or — at least one condition must pass for this rule to be applied (OR).
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// Fires controls at which lifecycle points this rule is applied.
	// Absent: fires at both admission and reconcile time.
	// fires.reconcile: false — admission only; reconciler skips this rule.
	Fires *FiresConfig `yaml:"fires,omitempty" json:"fires,omitempty"`
}

// MutationConfig holds all mutation rules for a CRD.
type MutationConfig struct {
	// Include is a path (relative to the catalog file) to a YAML file whose
	// top-level value is a list of MutationRule entries. Expanded at load
	// time — included rules come first, inline rules append after.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`

	// External declares HTTP calls made before mutation rules are applied.
	// Results are available in rule field:, default:, and override: expressions.
	// At admission time: all entries fire.
	// At reconcile time: only entries with fires.reconcile != false fire.
	External []ExternalCallSpec `yaml:"external,omitempty" json:"external,omitempty"`

	// Rules — applied in declaration order.
	Rules []MutationRule `yaml:"rules,omitempty" json:"rules,omitempty"`

	// MutateFirst — when true, mutation runs before validation at reconcile
	// time. Default true (mutate first, then validate valid objects).
	//
	// At admission time, mutation always runs first — this mirrors the
	// Kubernetes webhook ordering (MutatingWebhookConfiguration fires before
	// ValidatingWebhookConfiguration). This field only affects reconcile ordering.
	MutateFirst bool `yaml:"mutateFirst,omitempty" json:"mutateFirst,omitempty"`
}

// MutationChangeType describes the kind of mutation change for on mutation rule
// Currently supported - one of: override and default
type MutationChangeType string

const (
	OverrideMutationChangeType MutationChangeType = "override"
	DefaultMutationChangeType  MutationChangeType = "default"
	UnknownMutationChangeType  MutationChangeType = "unknown"
)

func (t MutationChangeType) String() string {
	return string(t)
}

// ChangeType returns the change type for this rule
func (r MutationRule) ChangeType() MutationChangeType {
	if r.Override != nil {
		return OverrideMutationChangeType
	}
	if r.Default != nil {
		return DefaultMutationChangeType
	}
	return UnknownMutationChangeType
}

// IsOverrideChangeType reports whether this rule is an override change type
func (r MutationRule) IsOverrideChangeType() bool {
	if r.Override != nil {
		return true
	}
	return false
}

// IsDefaultChangeType reports whether this rule is a default change type
func (r MutationRule) IsDefaultChangeType() bool {
	if !r.IsOverrideChangeType() && r.Default != nil {
		return true
	}
	return false
}

// IsValidChangeType reports true when at least default or override is defined
func (r MutationRule) IsValidChangeType() bool {
	switch r.ChangeType() {
	case OverrideMutationChangeType, DefaultMutationChangeType:
		return true
	default:
		return false
	}
}

// HasDefaultAndOverride reports whether both default and override are defined for this rule
func (r MutationRule) HasDefaultAndOverride() bool {
	return r.Override != nil && r.Default != nil
}

// MutationRuleFor returns the first matching mutation rule for a given field
func MutationRuleFor(rules []MutationRule, field string) MutationRule {
	if rules == nil && field != "" {
		return MutationRule{}
	}
	for _, rule := range rules {
		if rule.Field == field {
			return rule
		}
	}
	return MutationRule{}
}

// MutationRulesFor returns the all matching mutation rules for a given field
func MutationRulesFor(rules []MutationRule, field string) []MutationRule {
	if rules == nil && field != "" {
		return []MutationRule{}
	}

	result := []MutationRule{}
	for _, rule := range rules {
		if rule.Field == field {
			result = append(result, rule)
		}
	}
	return result
}

// HasAnyRules reports whether any mutation rule is configured
func (c *MutationConfig) HasAnyRules() bool {
	if c == nil {
		return false
	}
	return len(c.Rules) > 0
}

// AdmissionExternal returns all external calls — every entry fires at admission time.
func (c *MutationConfig) AdmissionExternal() []ExternalCallSpec {
	if c == nil {
		return nil
	}
	return c.External
}

// HasExternalCall reports whether there is any external call in the config
func (c *MutationConfig) HasExternalCall() bool {
	if c == nil {
		return false
	}
	return len(c.External) > 0
}

// ReconcileExternal returns only calls that fire at reconcile time.
// Entries with fires.reconcile: false are excluded.
func (c *MutationConfig) ReconcileExternal() []ExternalCallSpec {
	if c == nil {
		return nil
	}
	var out []ExternalCallSpec
	for _, e := range c.External {
		if e.Fires.FiresAtReconcile() {
			out = append(out, e)
		}
	}
	return out
}

// HasUniqueRule reports whether any rule sets unique: true.
// Always false for mutation — unique is a validation-only operator.
func (c *MutationConfig) HasUniqueRule() bool {
	return false
}

// References reports whether any rule's field, default or override reads
// the data root (for example "health" or "metrics"), directly or through a
// note.
func (c *MutationConfig) References(nr NoteRegistry, root string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Rules {
		if nr.References(r.Field, root) {
			return true
		}
		if s, ok := r.Default.(string); ok && nr.References(s, root) {
			return true
		}
		if s, ok := r.Override.(string); ok && nr.References(s, root) {
			return true
		}
	}
	return false
}

// ── AdmissionWebhookConfig ────────────────────────────────────────────────

// AdmissionWebhookConfig is the per-CRD admission webhook control block.
// Declared under spec.crds[].webhooks in the Catalog.
//
// Example:
//
//	website:
//	  webhooks:
//	  	validation: true   # intercept at admission — ValidatingWebhookConfiguration
//	  	mutation: true     # intercept at admission — MutatingWebhookConfiguration
//
// Both default to true when `security.webhooks` or ENABLE_ADMISSION_WEBHOOK=true
// and the corresponding validation/mutation block has rules declared.
// Set to false to opt a specific CRD out of admission interception
// while keeping its reconcile-time enforcement.
type AdmissionWebhookConfig struct {
	// Validation — include this CRD in the ValidatingWebhookConfiguration.
	// Default: true when validation rules are declared.
	Validation *bool `yaml:"validation,omitempty" json:"validation,omitempty"`

	// Mutation — include this CRD in the MutatingWebhookConfiguration.
	// Default: true when mutation rules are declared.
	Mutation *bool `yaml:"mutation,omitempty" json:"mutation,omitempty"`

	// Operations — which operations trigger the webhook.
	// Default: ["CREATE", "UPDATE"]
	// Valid values: CREATE, UPDATE, DELETE, CONNECT
	Operations []string `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// WebhookValidationEnabled reports whether admission-time validation is
// enabled for this CRD. True when Validation is nil or true.
func (w *AdmissionWebhookConfig) WebhookValidationEnabled() bool {
	if w == nil || w.Validation == nil {
		return true // default on
	}
	return *w.Validation
}

// WebhookMutationEnabled reports whether admission-time mutation is enabled.
func (w *AdmissionWebhookConfig) WebhookMutationEnabled() bool {
	if w == nil || w.Mutation == nil {
		return true // default on
	}
	return *w.Mutation
}

// EffectiveOperations returns the operations this webhook intercepts.
// Defaults to CREATE and UPDATE when not specified.
func (w *AdmissionWebhookConfig) EffectiveOperations() []string {
	if w == nil || len(w.Operations) == 0 {
		return []string{"CREATE", "UPDATE"}
	}
	return w.Operations
}

// ── AdmissionConfig ────────────────────────────────────────────────────────────

// AdmissionConfig groups all admission and versioning rules under crdEntry.admission:.
// It supersedes the top-level Validation, Mutation, Webhooks, and Conversion fields
// on CRDEntry. Both the new admission: block and the legacy top-level fields are
// accepted during migration — accessor methods check the new path first.
type AdmissionConfig struct {
	// Validation declares CEL/webhook validation rules evaluated at admission time.
	Validation *ValidationConfig `yaml:"validation,omitempty" json:"validation,omitempty"`

	// Mutation declares mutation rules applied to incoming CRs at admission time.
	Mutation *MutationConfig `yaml:"mutation,omitempty" json:"mutation,omitempty"`

	// Webhooks controls per-CRD admission webhook behaviour.
	Webhooks AdmissionWebhookConfig `yaml:"webhooks,omitempty" json:"webhooks,omitempty"`

	// Conversion handles multi-version CRD conversion via a webhook or built-in strategy.
	Conversion *CRDConversion `yaml:"conversion,omitempty" json:"conversion,omitempty"`
}

func (a *AdmissionConfig) Empty() bool { return a == nil }

// HasValidation reports whether validation rules are declared.
func (a *AdmissionConfig) HasValidation() bool {
	return a != nil && a.Validation != nil && len(a.Validation.Rules) > 0
}

// HasMutation reports whether mutation rules are declared.
func (a *AdmissionConfig) HasMutation() bool {
	return a != nil && a.Mutation != nil && len(a.Mutation.Rules) > 0
}

// HasConversion reports whether conversion is declared.
func (a *AdmissionConfig) HasConversion() bool {
	return a != nil && a.Conversion != nil
}
