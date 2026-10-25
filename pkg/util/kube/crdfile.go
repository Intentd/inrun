package kube

import (
	"fmt"
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"gopkg.in/yaml.v3"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	sigsyaml "sigs.k8s.io/yaml"
)

// ValidateSchemaPath reads the CRD YAML at crdPath and confirms that
// specPath (dot-separated, e.g. "spec.scaling.min") resolves to a leaf
// field in the CRD's OpenAPI schema.
func ValidateSchemaPath(crdPath, specPath string) error {
	schema, err := ReadSchemaFromCRDFile(crdPath)
	if err != nil {
		return fmt.Errorf("reading CRD %q: %w", crdPath, err)
	}
	if schema == nil {
		return nil // no schema — can't validate
	}

	const prefix = "spec."
	if !strings.HasPrefix(specPath, prefix) {
		specPath = prefix + specPath
	}

	fieldPath := strings.TrimPrefix(specPath, prefix)
	if fieldPath == "" {
		return fmt.Errorf("invalid schema path %q: expected a field under spec", specPath)
	}

	current := schema
	segments := strings.Split(fieldPath, ".")
	for i, seg := range segments {
		if seg == "" {
			return fmt.Errorf("invalid schema path %q: empty path segment", specPath)
		}

		if current.Type != "" && current.Type != "object" {
			return fmt.Errorf("path segment %q: parent is type %q, not an object", seg, current.Type)
		}

		if current.Properties == nil {
			return fmt.Errorf("path segment %q: no properties at this level", seg)
		}

		next, ok := current.Properties[seg]
		if !ok {
			keys := strings.Join(schemaPropKeys(current.Properties), ", ")
			return fmt.Errorf("path segment %q not found in CRD schema (available: %s)", seg, keys)
		}

		if i < len(segments)-1 && next.Type != "object" {
			return fmt.Errorf("path segment %q is type %q, not an object; cannot descend", seg, next.Type)
		}

		current = &next
	}

	if current.Type == "object" {
		return fmt.Errorf("path %q resolves to an object, not a leaf field", specPath)
	}

	return nil
}

// ReadSchemaFromCRDFile reads the CRD at path and returns its OpenAPI schema
// for the storage version, falling back to a served version when necessary.
// It returns nil when the selected version has no schema.
func ReadSchemaFromCRDFile(path string) (*apiextensions.JSONSchemaProps, error) {
	data, err := fileio.ReadLocal(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var doc apiextensionsv1.CustomResourceDefinition
	if err := sigsyaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if doc.Kind != "CustomResourceDefinition" {
		return nil, fmt.Errorf("expected kind: CustomResourceDefinition, got %q", doc.Kind)
	}

	version := selectCRDVersionV1(doc.Spec.Versions)
	if version == "" {
		return nil, fmt.Errorf("no served or storage version found")
	}

	for _, v := range doc.Spec.Versions {
		if v.Name != version {
			continue
		}
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			return nil, nil
		}
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v.Schema.OpenAPIV3Schema, internal, nil,
		); err != nil {
			return nil, fmt.Errorf("converting schema: %w", err)
		}
		specSchema, ok := internal.Properties["spec"]
		if !ok {
			return nil, fmt.Errorf("CRD schema has no spec property")
		}
		return &specSchema, nil
	}
	return nil, fmt.Errorf("selected CRD version %q not found", version)
}

// CRDFileHeader is the identity part of a CRD YAML: group, names, scope and
// versions. Read it with ReadCRDFileHeader; use ReadSchemaFromCRDFile for the
// OpenAPI schema.
type CRDFileHeader struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Group string `yaml:"group"`
		Scope string `yaml:"scope"`
		Names struct {
			Kind   string `yaml:"kind"`
			Plural string `yaml:"plural"`
		} `yaml:"names"`
		Versions []CRDFileVersion `yaml:"versions"`
	} `yaml:"spec"`
}

// CRDFileVersion is one entry of spec.versions.
type CRDFileVersion struct {
	Name    string `yaml:"name"`
	Served  bool   `yaml:"served"`
	Storage bool   `yaml:"storage"`
}

// ReadCRDFileHeader reads the CRD YAML at path and checks the fields every
// caller needs: kind, group, names.kind, names.plural, a usable version and a
// valid scope.
func ReadCRDFileHeader(path string) (*CRDFileHeader, error) {
	data, err := fileio.ReadLocal(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	var h CRDFileHeader
	if err := yaml.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	switch {
	case h.Kind != "CustomResourceDefinition":
		return nil, fmt.Errorf("expected kind: CustomResourceDefinition, got %q", h.Kind)
	case h.Spec.Group == "":
		return nil, fmt.Errorf("spec.group is missing")
	case h.Spec.Names.Kind == "":
		return nil, fmt.Errorf("spec.names.kind is missing")
	case h.Spec.Names.Plural == "":
		return nil, fmt.Errorf("spec.names.plural is missing")
	case len(h.Spec.Versions) == 0:
		return nil, fmt.Errorf("spec.versions is empty")
	case h.Version() == "":
		return nil, fmt.Errorf("no served or storage version found")
	}
	if _, err := h.Namespaced(); err != nil {
		return nil, err
	}
	return &h, nil
}

// Version picks the version to use: storage, else served, else the first.
func (h *CRDFileHeader) Version() string {
	for _, v := range h.Spec.Versions {
		if v.Storage {
			return v.Name
		}
	}
	for _, v := range h.Spec.Versions {
		if v.Served {
			return v.Name
		}
	}
	if len(h.Spec.Versions) > 0 {
		return h.Spec.Versions[0].Name
	}
	return ""
}

// APIVersion returns "group/version".
func (h *CRDFileHeader) APIVersion() string {
	return h.Spec.Group + "/" + h.Version()
}

// Namespaced reports whether the CRD is namespaced. An empty scope counts as
// namespaced.
func (h *CRDFileHeader) Namespaced() (bool, error) {
	switch h.Spec.Scope {
	case "", "Namespaced":
		return true, nil
	case "Cluster":
		return false, nil
	default:
		return false, fmt.Errorf("spec.scope %q must be Namespaced or Cluster", h.Spec.Scope)
	}
}

// selectCRDVersionV1 is CRDFileHeader.Version for the typed v1 CRD.
func selectCRDVersionV1(versions []apiextensionsv1.CustomResourceDefinitionVersion) string {
	for _, v := range versions {
		if v.Storage {
			return v.Name
		}
	}
	for _, v := range versions {
		if v.Served {
			return v.Name
		}
	}
	if len(versions) > 0 {
		return versions[0].Name
	}
	return ""
}

func schemaPropKeys(props map[string]apiextensions.JSONSchemaProps) []string {
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
