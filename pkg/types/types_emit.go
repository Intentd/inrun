// pkg/types/types_emit.go
package types

import "strings"

// EmitEventType mirrors corev1.EventType* for use in declarative emit declarations.
type EmitEventType string

const (
	// EmitEventTypeNormal mirrors corev1.EventTypeNormal — informational events.
	EmitEventTypeNormal EmitEventType = "Normal"
	// EmitEventTypeWarning mirrors corev1.EventTypeWarning — events requiring attention.
	EmitEventTypeWarning EmitEventType = "Warning"
)

// ValidEmitEventTypes returns all known EmitEventType values.
func ValidEmitEventTypes() []string {
	return []string{string(EmitEventTypeNormal), string(EmitEventTypeWarning)}
}

// EmitEventTypesJoined returns a comma-separated list of valid emit event types for error messages.
func EmitEventTypesJoined() string {
	return strings.Join(ValidEmitEventTypes(), ", ")
}

// IsValidEmitEventType reports whether t is a known EmitEventType value.
func IsValidEmitEventType(t EmitEventType) bool {
	switch t {
	case EmitEventTypeNormal, EmitEventTypeWarning:
		return true
	}
	return false
}

// EmitTrigger is the reconcile outcome that gates event emission.
type EmitTrigger string

const (
	// EmitTriggerAlways fires on every reconcile regardless of outcome. Default when On is empty.
	EmitTriggerAlways EmitTrigger = "always"
	// EmitTriggerSuccess fires only when the reconcile returned no error.
	EmitTriggerSuccess EmitTrigger = "success"
	// EmitTriggerFailure fires only when the reconcile returned an error.
	EmitTriggerFailure EmitTrigger = "failure"
)

// ValidEmitTriggers returns all known EmitTrigger values.
func ValidEmitTriggers() []string {
	return []string{string(EmitTriggerAlways), string(EmitTriggerSuccess), string(EmitTriggerFailure)}
}

// IsValidEmitTrigger reports whether s is a known EmitTrigger value.
func IsValidEmitTrigger(s string) bool {
	switch EmitTrigger(s) {
	case EmitTriggerAlways, EmitTriggerSuccess, EmitTriggerFailure:
		return true
	}
	return false
}

// EmitEventEntry declares one named event the runtime emits on behalf of this operator.
// Evaluated in postReconcile using the prepared resolver context.
type EmitEventEntry struct {
	// Type is the Kubernetes event type. Must be EmitEventTypeNormal or EmitEventTypeWarning.
	Type EmitEventType `yaml:"type" json:"type" validate:"required"`

	// Reason is the event reason field. Supports template expressions.
	Reason string `yaml:"reason" json:"reason" validate:"required"`

	// Message is the human-readable event message. Evaluated as a Go template
	// against the prepared resolver context.
	Message string `yaml:"message" json:"message" validate:"required"`

	// On declares which reconcile outcomes trigger this event.
	// Valid values: always, success, failure. Defaults to always when empty.
	On []EmitTrigger `yaml:"on,omitempty" json:"on,omitempty"`

	// When declares AND conditions — all must pass for the event to be emitted.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or declares OR conditions — any passing condition emits the event.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`
}

// EmitOn reports whether this entry should fire given the reconcile outcome.
// An empty On list is treated as always.
func (e *EmitEventEntry) EmitOn(reconcileErr error) bool {
	if len(e.On) == 0 {
		return true
	}
	for _, trigger := range e.On {
		switch trigger {
		case EmitTriggerAlways:
			return true
		case EmitTriggerSuccess:
			if reconcileErr == nil {
				return true
			}
		case EmitTriggerFailure:
			if reconcileErr != nil {
				return true
			}
		}
	}
	return false
}

// EmitConfig writes the reconciler's conclusions back to the CR and the event stream.
// status: declares fields patched onto the CR after every reconcile.
// events: declares named structured events emitted on lifecycle transitions.
type EmitConfig struct {
	// Status declares declarative status fields written after every reconcile.
	Status *StatusConfig `yaml:"status,omitempty" json:"status,omitempty"`

	// Events declares named Kubernetes events emitted in postReconcile.
	// Map key is the event name — a stable identifier used for deduplication.
	Events map[string]*EmitEventEntry `yaml:"events,omitempty" json:"events,omitempty"`
}

// HasStatus reports whether a status declaration is present.
func (e *EmitConfig) HasStatus() bool {
	return e != nil && e.Status != nil
}

// HasEvents reports whether any event declarations are present.
func (e *EmitConfig) HasEvents() bool {
	return e != nil && len(e.Events) > 0
}
