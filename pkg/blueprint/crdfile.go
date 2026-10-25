package blueprint

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/kube"
	"gopkg.in/yaml.v3"
)

// PopulateAPITypesFromCRDFile reads the CRD YAML declared in entry.CRDFile and
// overwrites entry.APITypes with the values parsed from the file.
//
// crdFile is the source of truth — any apiTypes already declared on the entry
// are replaced. This is called in the enrichment loop in BuildRuntimeBlueprint,
// before EnrichCRDEntry, so the entry is fully specified by the time
// isFullySpecified runs.
//
// Remote URLs (http/https) are left untouched — kubectl handles those at runtime.
func PopulateAPITypesFromCRDFile(entry *types.CRDEntry, blueprintDir string) error {
	path := entry.CRDFile

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return nil
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(blueprintDir, path)
	}

	h, err := kube.ReadCRDFileHeader(path)
	if err != nil {
		return fmt.Errorf("reading crdFile %q: %w", entry.CRDFile, err)
	}
	apiTypes := types.APITypesFromCRDHeader(h)

	// The file's scope wins; a declared namespaced: must agree with it.
	namespaced, _ := h.Namespaced() // checked by ReadCRDFileHeader
	if entry.Namespaced != nil && *entry.Namespaced != namespaced {
		return fmt.Errorf("namespaced: %t does not match crdFile scope %q", *entry.Namespaced, h.Spec.Scope)
	}
	entry.Namespaced = &namespaced

	// Keep the path for the validator; CRDFile is cleared after load.
	entry.ResolvedCRDFile = path

	// Preserve typed-mode fields (Object, List, Alias, Location) — those come
	// from the user's apiTypes declaration and are not present in the CRD YAML.
	apiTypes.Object = entry.APITypes.Object
	apiTypes.List = entry.APITypes.List
	apiTypes.Alias = entry.APITypes.Alias
	apiTypes.Location = entry.APITypes.Location
	apiTypes.APIPath = entry.APITypes.APIPath

	entry.APITypes = apiTypes
	return nil
}

// PopulateMigrateTargetFromCRDFile fills migrate.to's apiVersion, kind and
// plural from migrate.to.crdFile, the same way crdFile fills apiTypes.
// Declared values must match the file. Remote URLs are left untouched.
func PopulateMigrateTargetFromCRDFile(entry *types.CRDEntry, blueprintDir string) error {
	if entry.Migrate == nil || entry.Migrate.To.CRDFile == "" {
		return nil
	}
	to := &entry.Migrate.To
	path := to.CRDFile
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(blueprintDir, path)
	}

	file, err := readAPITypesFromCRDFile(path)
	if err != nil {
		return fmt.Errorf("reading migrate.to.crdFile %q: %w", to.CRDFile, err)
	}
	apiVersion := file.Group + "/" + file.Version
	for _, c := range []struct{ field, declared, file string }{
		{"apiVersion", to.APIVersion, apiVersion},
		{"kind", to.Kind, file.Kind},
		{"plural", to.Plural, file.Plural},
	} {
		if c.declared != "" && c.declared != c.file {
			return fmt.Errorf("migrate.to.%s %q does not match migrate.to.crdFile (%q)", c.field, c.declared, c.file)
		}
	}
	to.APIVersion, to.Kind, to.Plural = apiVersion, file.Kind, file.Plural
	to.ResolvedCRDFile = path
	return nil
}

// ValidateSchemaPath reads the CRD YAML at crdPath and confirms that
// specPath resolves to a leaf field in the CRD's OpenAPI schema.
func ValidateSchemaPath(crdPath, specPath string) error {
	return kube.ValidateSchemaPath(crdPath, specPath)
}

// readAPITypesFromCRDFile reads a CRD YAML file and extracts APITypes.
func readAPITypesFromCRDFile(path string) (*types.APITypes, error) {
	h, err := kube.ReadCRDFileHeader(path)
	if err != nil {
		return nil, err
	}
	a := types.APITypesFromCRDHeader(h)
	return &a, nil
}

// ResolveCRDFiles reads the blueprint YAML at path, resolves every crdFile
// reference to inline apiTypes, removes the crdFile key, and returns the
// rewritten YAML. The result is safe to embed in a bundle ConfigMap — the
// Inrun runtime receives concrete apiTypes and never needs to read the file.
func ResolveCRDFiles(path string) ([]byte, error) {
	data, err := readLocal(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	spec, _ := raw["spec"].(map[string]interface{})
	if spec == nil {
		return data, nil
	}
	crds, _ := spec["crds"].(map[string]interface{})
	if crds == nil {
		return data, nil
	}

	dir := filepath.Dir(path)
	for name, val := range crds {
		entry, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		if err := resolveMigrateCRDFile(entry, dir); err != nil {
			return nil, fmt.Errorf("CRD %q: %w", name, err)
		}
		crds[name] = entry

		crdFile, _ := entry["crdFile"].(string)
		if crdFile == "" {
			continue
		}

		absPath := crdFile
		if !filepath.IsAbs(crdFile) && !strings.HasPrefix(crdFile, "http") {
			absPath = filepath.Join(dir, crdFile)
		}

		h, err := kube.ReadCRDFileHeader(absPath)
		if err != nil {
			return nil, fmt.Errorf("CRD %q: %w", name, err)
		}
		apiTypes := types.APITypesFromCRDHeader(h)
		namespaced, _ := h.Namespaced()
		entry["namespaced"] = namespaced

		// Merge: start from whatever apiTypes the user already declared so that
		// typed-mode fields (location, object, objectList, alias, apiPath) are
		// preserved. Overwrite only the structural fields that crdFile is
		// authoritative for (group, version, kind, plural).
		existing, _ := entry["apiTypes"].(map[string]interface{})
		if existing == nil {
			existing = map[string]interface{}{}
		}
		existing["group"] = apiTypes.Group
		existing["version"] = apiTypes.Version
		existing["kind"] = apiTypes.Kind
		existing["plural"] = apiTypes.Plural
		entry["apiTypes"] = existing
		delete(entry, "crdFile")
		crds[name] = entry
	}

	spec["crds"] = crds
	raw["spec"] = spec

	out, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshaling resolved blueprint: %w", err)
	}
	return out, nil
}

// resolveMigrateCRDFile inlines migrate.to.crdFile in a raw blueprint entry
// as apiVersion, kind and plural, and removes the crdFile key.
func resolveMigrateCRDFile(entry map[string]interface{}, dir string) error {
	mig, _ := entry["migrate"].(map[string]interface{})
	to, _ := mig["to"].(map[string]interface{})
	crdFile, _ := to["crdFile"].(string)
	if crdFile == "" || strings.HasPrefix(crdFile, "http") {
		return nil
	}
	path := crdFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	file, err := readAPITypesFromCRDFile(path)
	if err != nil {
		return fmt.Errorf("reading migrate.to.crdFile %q: %w", crdFile, err)
	}
	to["apiVersion"] = file.Group + "/" + file.Version
	to["kind"] = file.Kind
	to["plural"] = file.Plural
	delete(to, "crdFile")
	return nil
}
