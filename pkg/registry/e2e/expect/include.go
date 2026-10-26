package expect

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert/yaml"
)

// ExpandIncludes resolves include: entries in the expect list, loading each
// referenced file and splicing its entries in place. Relative paths are resolved
// against baseDir. Nested includes are not supported.
func ExpandIncludes(expects []types.E2EExpectation, baseDir string) ([]types.E2EExpectation, error) {
	var result []types.E2EExpectation
	for _, exp := range expects {
		if exp.Include == "" {
			result = append(result, exp)
			continue
		}
		path := exp.Include
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		data, err := readLocal(path)
		if err != nil {
			return nil, fmt.Errorf("include %s: %w", exp.Include, err)
		}
		var wrapper struct {
			Expect []types.E2EExpectation `yaml:"expect"`
		}
		if err := yaml.Unmarshal(data, &wrapper); err != nil {
			return nil, fmt.Errorf("include %s: %w", exp.Include, err)
		}
		includeDir := filepath.Dir(path)
		for i := range wrapper.Expect {
			wrapper.Expect[i].WorkDir = includeDir
		}
		result = append(result, wrapper.Expect...)
	}
	for i := range result {
		dir := baseDir
		if result[i].WorkDir != "" {
			dir = result[i].WorkDir
		}
		external, err := expandExternalIncludes(result[i].External, dir)
		if err != nil {
			return nil, fmt.Errorf("expect %q: %w", result[i].Name, err)
		}
		result[i].External = external
	}
	return result, nil
}

// expandExternalIncludes replaces each include: entry in checks with the
// calls: its file lists, resolved against dir, as a Blueprint's calls are
// expanded. The file's calls may carry their own jq and assertions; a
// Blueprint's calls file is one without them.
func expandExternalIncludes(checks []types.E2EExternalCheck, dir string) ([]types.E2EExternalCheck, error) {
	var out []types.E2EExternalCheck
	for _, c := range checks {
		if c.Include == "" {
			out = append(out, c)
			continue
		}
		path := c.Include
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		data, err := readLocal(path)
		if err != nil {
			return nil, fmt.Errorf("reading external include %q: %w", c.Include, err)
		}
		var f struct {
			Calls []types.E2EExternalCheck `yaml:"calls"`
		}
		if err := strictUnmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parsing external include %q: %w", c.Include, err)
		}
		out = append(out, f.Calls...)
	}
	return out, nil
}
