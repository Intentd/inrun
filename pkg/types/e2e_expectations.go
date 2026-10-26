package types

import (
	"fmt"
	"path/filepath"
)

// E2EExpectations is a suite's expect list, or an include file's.
type E2EExpectations []E2EExpectation

// Names returns each expectation's name, in order.
func (e E2EExpectations) Names() []string {
	out := make([]string, len(e))
	for i, exp := range e {
		out[i] = exp.Name
	}
	return out
}

// ExpandIncludes replaces each include: entry with the expectations its file
// lists, run from that file's directory, and each external include with its
// calls. Relative paths resolve against baseDir. Nested includes are not
// supported.
func (e E2EExpectations) ExpandIncludes(baseDir string) (E2EExpectations, error) {
	var result E2EExpectations
	for _, exp := range e {
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
		var file E2EExpectInclude
		if err := strictUnmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("include %s: %w", exp.Include, err)
		}
		includeDir := filepath.Dir(path)
		for i := range file.Expect {
			file.Expect[i].WorkDir = includeDir
		}
		result = append(result, file.Expect...)
	}
	for i := range result {
		external, err := expandExternalIncludes(result[i].External, result[i].Dir(baseDir))
		if err != nil {
			return nil, fmt.Errorf("expect %q: %w", result[i].Name, err)
		}
		result[i].External = external
	}
	return result, nil
}

// expandExternalIncludes replaces each include: entry in checks with the
// calls its file lists, resolved against dir.
func expandExternalIncludes(checks E2EExternalChecks, dir string) (E2EExternalChecks, error) {
	var out E2EExternalChecks
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
		var f E2EExternalInclude
		if err := strictUnmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parsing external include %q: %w", c.Include, err)
		}
		out = append(out, f.Calls...)
	}
	return out, nil
}
