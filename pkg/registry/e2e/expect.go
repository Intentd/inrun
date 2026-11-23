package e2e

import (
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/types"
)

// specPathRel names the spec file relative to the pattern root, the way
// Files: lists it: "test/e2e.yaml".
func (r *Runner) specPathRel() string {
	if r.blueprintFile == "" {
		return filepath.Base(r.e2eFile)
	}
	base := filepath.Dir(r.blueprintFile)
	if rel, err := filepath.Rel(base, r.e2eFile); err == nil {
		return rel
	}
	return filepath.Base(r.e2eFile)
}

// loadCRData reads spec.CR into a map, for when:/or: conditions. nil when
// spec is nil, CR is unset, or the file cannot be read.
func loadCRData(spec *types.E2ESpec, workDir string) map[string]interface{} {
	if spec == nil || spec.CR == "" {
		return nil
	}
	crPath := spec.CR
	if !filepath.IsAbs(crPath) {
		crPath = filepath.Join(workDir, crPath)
	}
	data, err := readLocal(crPath)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	_ = yaml.Unmarshal(data, &out)
	return out
}
