package emit

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/intentd/inrun/pkg/types"
)

// validateEmit checks operatorBox.emit.events across all enabled CRDs.
//
// Enforces:
//  1. type must be a valid EmitEventType (Normal or Warning).
//  2. reason must be non-empty.
//  3. message must be non-empty and a valid Go template when it contains "{{".
//  4. when/or condition field values are valid Go templates when they contain "{{".
func (c *checker) validateEmit() error {
	funcMap := buildFuncMapForValidation(c.Cat.Notes)
	for crdName, crd := range c.Cat.EnabledCRDs() {
		for name, entry := range crd.Box().EmitEntries() {
			if err := validateEmitEventEntry(crdName, name, entry, funcMap); err != nil {
				return err
			}
		}
		if err := validateStatusConfig(crdName, "operatorBox.status", crd.Box().EffectiveStatus()); err != nil {
			return err
		}
		if crd.Box().Emit != nil {
			if err := validateStatusConfig(crdName, "operatorBox.emit.status", crd.Box().Emit.Status); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEmitEventEntry(crdName, name string, entry *types.EmitEventEntry, funcMap template.FuncMap) error {
	if entry == nil {
		return nil
	}
	if !types.IsValidEmitEventType(entry.Type) {
		return fmt.Errorf("%s crd %q: emit.events[%q]: type %q is invalid — valid values: %s",
			failureMark(), crdName, name, entry.Type, types.EmitEventTypesJoined())
	}
	for _, trigger := range entry.On {
		if !types.IsValidEmitTrigger(string(trigger)) {
			return fmt.Errorf("%s crd %q: emit.events[%q]: on %q is invalid — valid values: %s",
				failureMark(), crdName, name, trigger, strings.Join(types.ValidEmitTriggers(), ", "))
		}
	}
	if entry.Reason == "" {
		return fmt.Errorf("%s crd %q: emit.events[%q]: reason must not be empty",
			failureMark(), crdName, name)
	}
	if entry.Message == "" {
		return fmt.Errorf("%s crd %q: emit.events[%q]: message must not be empty",
			failureMark(), crdName, name)
	}
	if isTemplate(entry.Message) {
		if err := validateTemplate("emit.events", crdName, name, "message", entry.Message, funcMap); err != nil {
			return err
		}
	}
	for i, c := range entry.When {
		if isTemplate(c.Field) {
			if err := validateTemplate("emit.events", crdName, name, fmt.Sprintf("when[%d].field", i), c.Field, funcMap); err != nil {
				return err
			}
		}
	}
	for i, c := range entry.Or {
		if isTemplate(c.Field) {
			if err := validateTemplate("emit.events", crdName, name, fmt.Sprintf("or[%d].field", i), c.Field, funcMap); err != nil {
				return err
			}
		}
	}
	return nil
}
