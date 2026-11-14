// pkg/types/conditions.go
package types

import "github.com/intentd/tools/condition"

// Condition is one test against the CR or runtime state. Field is a
// dot-notation path (spec.environment, metadata.name, metrics.queueDepth) and
// the comparison fields say what its value must be.
//
// On a template entry, under onCreate, onReconcile or onDelete, conditions
// decide whether that entry is written this pass:
//
//   - when: lists conditions that must all pass.
//
//   - or: lists conditions of which at least one must pass.
//
//   - With both set, both must hold; with neither, the entry always runs.
//
//     when:
//
//   - field: spec.environment
//     equals: production
//     or:
//
//   - field: spec.tier
//     equals: gold
//
//   - field: spec.replicas
//     greaterThan: "3"
//
// An entry whose conditions fail is skipped, which is not an error. If the
// entry is kept in sync (onReconcile, or reconcile: true) the object it wrote
// is also removed, as long as Inrun owns it and no other passing entry writes
// the same name. Jobs are the exception: they are never removed.
//
// Every other when: and or: in a Blueprint (autoscale, call: steps, hooks,
// admission, enrichment, emit, status, cron and e2e) uses this type and
// combines the two lists the same way.
type Condition struct {
	condition.Condition `yaml:",inline"`

	// Source is the HTTP fallback for cross-binary metric observation.
	// Only used when field is a cross.<crd>.metrics.* field and the CRD
	// is not registered in GlobalCrossMetricsRegistry (different binary).
	// The endpoint must be the remote operator's /blueprint/{crd} URL.
	//
	//   when:
	//     - field: cross.managed-database.metrics.queueDepth
	//       greaterThan: "500"
	//       source:
	//         host: "http://inrun-database-operator:8080"
	// 		   crd: managed-database
	//   when:
	//     - field: cross.managed-database.metrics.queueDepth
	//       greaterThan: "500"
	//       source:
	//         endpoint: "http://non-inrun-database-operator:8080/api/managed-database/metrics"
	Source *CrossSource `yaml:"source,omitempty" json:"source,omitempty"`
}

type Conditions []Condition

// Unwrap returns the conditions as the condition package evaluates them,
// without Inrun's Source; nil when there are none.
func (cs Conditions) Unwrap() condition.Conditions {
	if len(cs) == 0 {
		return nil
	}
	out := make(condition.Conditions, len(cs))
	for i, c := range cs {
		out[i] = c.Condition
	}
	return out
}

// ConditionOperator defines how a condition's field is compared to its value.
type ConditionOperator = condition.Operator

const (
	ConditionEquals      = condition.OpEquals
	ConditionNotEquals   = condition.OpNotEquals
	ConditionContains    = condition.OpContains
	ConditionNotContains = condition.OpNotContains
	ConditionPrefix      = condition.OpPrefix
	ConditionNotPrefix   = condition.OpNotPrefix
	ConditionSuffix      = condition.OpSuffix
	ConditionNotSuffix   = condition.OpNotSuffix
	ConditionExists      = condition.OpExists
	ConditionNotExists   = condition.OpNotExists
	ConditionGt          = condition.OpGt
	ConditionLt          = condition.OpLt
	ConditionGte         = condition.OpGte
	ConditionLte         = condition.OpLte
	ConditionIn          = condition.OpIn
	ConditionNotIn       = condition.OpNotIn
	ConditionRegex       = condition.OpRegex
	ConditionBetween     = condition.OpBetween
	ConditionNotBetween  = condition.OpNotBetween
	ConditionTypeOf      = condition.OpTypeOf

	// ConditionUnique — field value must not match any other existing
	// instance of this CRD (the CR being evaluated is excluded from its own
	// check). Works the same way in validation.rules and in when:/or:.
	//
	// Enforced at reconcile time via a live List() the reconciler injects
	// (pkg/runtime/coordinator/prepare/uniqueness.go) — the authoritative
	// check. Also enforced at admission time, as a best-effort early
	// rejection: the gateway queries the runtime's own informer cache over
	// HTTP. A stale cache there just means a duplicate slips through
	// admission and gets caught on the next reconcile.
	//
	//	validation:
	//	  rules:
	//	    - field: spec.domain
	//	      unique: true
	//	      message: "spec.domain must be unique across all instances"
	//	      action: deny
	//
	// Always passes wherever no checker is injected — e2e, simulate without
	// a seeded fixture, and any other context without live CRD access.
	ConditionUnique = condition.OpUnique
)

// FieldType is a value's type as typeOf names it: the names the typeOf
// template function returns.
type FieldType = condition.FieldType

const (
	FieldTypeMap    = condition.FieldTypeMap
	FieldTypeSlice  = condition.FieldTypeSlice
	FieldTypeString = condition.FieldTypeString
	FieldTypeNumber = condition.FieldTypeNumber
	FieldTypeBool   = condition.FieldTypeBool
	FieldTypeNull   = condition.FieldTypeNull
)

// FieldTypes is every FieldType, for validation and messages.
var FieldTypes = condition.FieldTypes

// TimeWindow declares a clock-based active window.
type TimeWindow = condition.TimeWindow

// DayOfWeekCondition declares which days the condition is active.
type DayOfWeekCondition = condition.DayOfWeek
