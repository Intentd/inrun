package catalog

import (
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
)

// ParseFile loads and enriches a Catalog from a single YAML file path.
// Uses a default config — suitable for CLI tools (plan, simulate) that do not
// need the full operator runtime.
func ParseFile(path string) (*Catalog, error) {
	kfg := config.NewDefaultConfig()
	m := merger.New(path)
	if err := m.Merge(); err != nil {
		return nil, fmt.Errorf("merging %q: %w", path, err)
	}
	k := &Catalog{}
	if _, err := k.BuildRuntimeCatalog(kfg, m); err != nil {
		return nil, err
	}
	return k, nil
}

// ParseBytes loads and enriches a Catalog from raw YAML bytes.
// dir is used as the base directory for resolving relative paths (e.g. crdFile).
// Pass "." when no specific directory context is available.
func ParseBytes(data []byte, dir string) (*Catalog, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	tmp, err := os.CreateTemp(dir, "inrun-catalog-*.yaml")
	if err != nil {
		tmp, err = os.CreateTemp("", "inrun-catalog-*.yaml")
		if err != nil {
			return nil, fmt.Errorf("creating temp file: %w", err)
		}
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp catalog: %w", err)
	}
	tmp.Close()
	return ParseFile(tmp.Name())
}
