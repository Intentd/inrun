package types

import (
	"fmt"

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
	When []Condition `yaml:"when,omitempty"`
	Or   []Condition `yaml:"or,omitempty"`
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
			When []Condition `yaml:"when"`
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

// MarshalYAML serializes back to the shorthand form when there are no
// conditions, and to the struct form when conditions are present.
func (e EnrichTarget) MarshalYAML() (interface{}, error) {
	if len(e.When) == 0 && len(e.Or) == 0 {
		return e.Key, nil
	}
	type body struct {
		When []Condition `yaml:"when,omitempty"`
		Or   []Condition `yaml:"or,omitempty"`
	}
	return map[string]body{
		e.Key: {When: e.When, Or: e.Or},
	}, nil
}
