// pkg/types/types_dependson.go
package types

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// ── DependsOn types ───────────────────────────────────────────────────────────

// DependsOnCondition is the value in the dependsOn map.
// Condition values: "started" (workers running) or "healthy" (running + consecutive failures = 0).
type DependsOnCondition struct {
	// Condition — started (the CRD's workers are running) or healthy (running
	// with no consecutive failures). Default: started.
	Condition string `yaml:"condition" json:"condition"`
}

// UnmarshalYAML handles Format 2 (scalar) and Format 3 (map) for a single dependency value.
//
//	database: healthy          ← Format 2: scalar
//	database:                  ← Format 3: map
//	  condition: healthy
func (d *DependsOnCondition) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.MappingNode {
		type plain DependsOnCondition
		return value.Decode((*plain)(d))
	}
	if value.Kind == yaml.ScalarNode {
		d.Condition = value.Value
		return nil
	}
	return fmt.Errorf("dependsOn value must be a string or map, got kind %v", value.Kind)
}

// DependsOnMap is the internal representation of all dependsOn formats.
// All three YAML formats unmarshal into this type.
type DependsOnMap map[string]DependsOnCondition

// UnmarshalYAML handles all three dependsOn formats:
//
//	Format 1 — list (condition defaults to "started"):
//	  dependsOn:
//	    - database
//
//	Format 2 — key-value map (condition explicit):
//	  dependsOn:
//	    database: healthy
//
//	Format 3 — full map:
//	  dependsOn:
//	    database:
//	      condition: healthy
func (m *DependsOnMap) UnmarshalYAML(value *yaml.Node) error {
	*m = make(DependsOnMap)

	// Format 1: sequence (list of names) → condition = "started"
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			if item.Kind == yaml.ScalarNode {
				(*m)[item.Value] = DependsOnCondition{Condition: "started"}
			}
		}
		return nil
	}

	// Format 2 + 3: mapping node
	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			key := value.Content[i].Value
			val := value.Content[i+1]

			var cond DependsOnCondition
			if err := val.Decode(&cond); err != nil {
				return fmt.Errorf("dependsOn[%s]: %w", key, err)
			}
			if cond.Condition == "" {
				cond.Condition = string(DependencyConditionStarted)
			}
			(*m)[key] = cond
		}
		return nil
	}

	return fmt.Errorf("dependsOn must be a list or map")
}

// Names returns the dependency names in sorted order.
func (m DependsOnMap) Names() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ConditionHealthy returns true if the dependency condition is healthy
func (m DependsOnMap) ConditionHealthy(name string) bool {
	cond, ok := m[name]
	return ok && cond.Condition == string(DependencyConditionHealthy)
}

// ConditionStarted returns true if the dependency condition is started
func (m DependsOnMap) ConditionStarted(name string) bool {
	cond, ok := m[name]
	return ok && cond.Condition == string(DependencyConditionStarted)
}
