package types

import (
	"fmt"

	"github.com/intentd/tools/schemagen"
	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

// EnrichTarget is one enrichment declaration in a CRD's enrich: list.
//
// Shorthand (always enriches):
//
//	enrich:
//	  - pods
//
// Struct form (conditional enrichment):
//
//	enrich:
//	  - events:
//	      when:
//	        - field: "{{ allReplicasReady .children.deployment }}"
//	          equals: "false"
//	  - replicasets:
//	      or:
//	        - field: spec.debug
//	          equals: "true"
type EnrichTarget struct {
	// Key is the enrichment target name (pods, events, pvcs, etc.)
	Key string

	// When gates this enrichment using AND semantics. Template expressions
	// in field: values are evaluated against the resolver's full data map
	// so KEL's functions (allReplicasReady, hasCrashingPod, …) have their inputs.
	// Empty: always enriches (same as shorthand).
	When Conditions `yaml:"when,omitempty"`
	// Or — enrich when at least one condition passes, alongside When.
	Or Conditions `yaml:"or,omitempty"`
}

// UnmarshalYAML handles both shorthand ("pods") and struct form:
//
//   - pods              → EnrichTarget{Key: "pods"}
//   - events:           → EnrichTarget{Key: "events", When: [...]}
//     when: [...]
func (e *EnrichTarget) UnmarshalYAML(value *yaml.Node) error {
	// Scalar: "pods" → EnrichTarget{Key: "pods"}
	if value.Kind == yaml.ScalarNode {
		e.Key = value.Value
		return nil
	}

	// Mapping: first key is the target name, value is the condition block.
	if value.Kind == yaml.MappingNode {
		if len(value.Content) < 2 {
			return fmt.Errorf("enrich target mapping must have exactly one key")
		}
		e.Key = value.Content[0].Value
		var body struct {
			When Conditions  `yaml:"when"`
			Or   []Condition `yaml:"or"`
		}
		if err := value.Content[1].Decode(&body); err != nil {
			return fmt.Errorf("enrich target %q: %w", e.Key, err)
		}
		e.When = body.When
		e.Or = body.Or
		return nil
	}

	return fmt.Errorf("enrich target must be a string or mapping, got node kind %d", value.Kind)
}

// JSONSchemaExtend is what UnmarshalYAML reads: an enrichment's name, or one
// key, the name, holding when/or gates.
func (EnrichTarget) JSONSchemaExtend(s *jsonschema.Schema) {
	one := uint64(1)
	gate := jsonschema.NewProperties()
	gate.Set("when", &jsonschema.Schema{Type: "array", Items: schemagen.Ref("Condition"), Description: "Enrich when every condition passes"})
	gate.Set("or", &jsonschema.Schema{Type: "array", Items: schemagen.Ref("Condition"), Description: "Enrich when at least one condition passes"})
	schemagen.Replace(s, &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
		schemagen.Str("An enrichment to turn on, always: pods, events, replicasets, ..."),
		{
			Type: "object", MinProperties: &one, MaxProperties: &one,
			AdditionalProperties: &jsonschema.Schema{
				Type: "object", Properties: gate, AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}})
}

// MarshalYAML serializes back to the shorthand form when there are no
// conditions, and to the struct form when conditions are present.
func (e EnrichTarget) MarshalYAML() (interface{}, error) {
	if len(e.When) == 0 && len(e.Or) == 0 {
		return e.Key, nil
	}
	type body struct {
		When Conditions  `yaml:"when,omitempty"`
		Or   []Condition `yaml:"or,omitempty"`
	}
	return map[string]body{
		e.Key: {When: e.When, Or: e.Or},
	}, nil
}
