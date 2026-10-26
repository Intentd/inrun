package simulate

import (
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/stretchr/testify/assert/yaml"
)

// DiscoverSimulateFiles walks root recursively and returns paths to all
// simulate.yaml leaf files (files with a spec:, not pure aggregators).
// Results are sorted for deterministic order.
func DiscoverSimulateFiles(root string, skip []string) ([]string, error) {
	return fileio.DiscoverFiles(root, registry.FileSimulate, skip, isSimulatePureAggregator)
}

// isSimulatePureAggregator returns true when the file has imports but no spec.
func isSimulatePureAggregator(path string) bool {
	data, err := fileio.ReadLocal(path)
	if err != nil {
		return false
	}
	var doc struct {
		Imports []string `yaml:"imports"`
		Spec    *struct {
			Blueprint string `yaml:"blueprint"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	return len(doc.Imports) > 0 && doc.Spec == nil
}
