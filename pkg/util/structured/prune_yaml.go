//go:build !runtime && !gateway

package structured

import "gopkg.in/yaml.v3"

func PruneEmptyYAML(data []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	pruneNode(&node)
	return yaml.Marshal(&node)
}

func pruneNode(node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for i := range node.Content {
			pruneNode(node.Content[i])
		}

	case yaml.MappingNode:
		// Filter out empty values
		filtered := make([]*yaml.Node, 0, len(node.Content))
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			// Check if value is empty (nil, empty sequence, empty mapping)
			if !isEmptyValue(value) {
				filtered = append(filtered, key, value)
				pruneNode(value) // recurse into kept values
			}
		}
		node.Content = filtered

	case yaml.SequenceNode:
		for i := range node.Content {
			pruneNode(node.Content[i])
		}
	}
}

func isEmptyValue(node *yaml.Node) bool {
	if node == nil {
		return true
	}

	switch node.Kind {
	case yaml.ScalarNode:
		// Empty string or zero duration/numbers
		return node.Value == "" || node.Value == "0" || node.Value == "0s" || node.Value == "null"
	case yaml.SequenceNode:
		return len(node.Content) == 0
	case yaml.MappingNode:
		return len(node.Content) == 0
	default:
		return false
	}
}
