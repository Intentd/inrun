package e2e

import (
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/fileio"
	"gopkg.in/yaml.v3"
)

// DiscoverE2EFiles walks root recursively and returns paths to all *e2e.yaml
// files that are not pure aggregators (files with imports: but no spec:).
// Results are sorted for deterministic order. skip is a list of glob patterns
// (matched against the full path); any file whose path matches is excluded.
func DiscoverE2EFiles(root string, skip []string) ([]string, error) {
	return fileio.DiscoverFiles(root, registry.FileE2E, skip, isE2EPureAggregatorFile)
}

// BuildDiscoveryE2E constructs an in-memory E2E aggregator from a list of
// discovered file paths. wait is injected on every import except the first.
func BuildDiscoveryE2E(paths []string, wait string) types.E2E {
	imports := make([]types.E2EImport, len(paths))
	for i, p := range paths {
		imp := types.E2EImport{Path: p}
		if i > 0 && wait != "" {
			imp.Wait = wait
		}
		imports[i] = imp
	}
	return types.E2E{
		APIVersion: "inrun.dev/v1",
		Kind:       "E2E",
		Metadata:   types.E2EMeta{Name: "discovered-suite"},
		Imports:    imports,
	}
}

// isE2EPureAggregatorFile reads the file and returns true if it has imports but no spec.
func isE2EPureAggregatorFile(path string) bool {
	data, err := readLocal(path)
	if err != nil {
		return false
	}
	var doc struct {
		Imports []interface{} `yaml:"imports"`
		Spec    struct {
			Blueprint string   `yaml:"blueprint"`
			CR        string   `yaml:"cr"`
			CRFiles   []string `yaml:"crFiles"`
			Custom    *struct {
				Target string `yaml:"target"`
			} `yaml:"custom"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}

	hasSpec := doc.Spec.Blueprint != "" ||
		doc.Spec.CR != "" || len(doc.Spec.CRFiles) > 0 ||
		(doc.Spec.Custom != nil && doc.Spec.Custom.Target != "")

	return len(doc.Imports) > 0 && !hasSpec
}
