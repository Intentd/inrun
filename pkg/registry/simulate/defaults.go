package simulate

import (
	"bytes"
	"fmt"

	"github.com/intentd/inrun/pkg/util/fileio"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// crdFacts is what the in-memory cluster takes from the CRD manifests the
// envtest backend installs: what the API server would do with each kind.
type crdFacts struct {
	// Defaults is, per kind and version, the defaulting the API server
	// applies: the default: values in its structural schema, applied with
	// the API server's own code — on the CR when it is created, and on every
	// write of the kind ((*kubefake.Kubeclient).SetDefaults) — so a field the schema
	// defaults is there in both backends.
	Defaults map[schema.GroupVersionKind]func(map[string]interface{})
	// Scopes is, per kind, whether it is namespaced (spec.scope), so the
	// fake's REST mapper sends a cluster-scoped kind's requests cluster-wide
	// ((*kubefake.Kubeclient).SetScope).
	Scopes map[schema.GroupKind]bool
}

// readCRDs reads the CRD manifests at paths.
//
// A file that cannot be read or parsed, a document that is not a CRD, and a
// version without a structural schema contribute nothing: the in-memory run
// has never needed CRD files, and still runs without them.
func readCRDs(paths []string) crdFacts {
	out := crdFacts{
		Defaults: map[schema.GroupVersionKind]func(map[string]interface{}){},
		Scopes:   map[schema.GroupKind]bool{},
	}
	for _, path := range paths {
		data, err := fileio.ReadLocal(path)
		if err != nil {
			continue
		}
		dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			var crd apiextensionsv1.CustomResourceDefinition
			if err := dec.Decode(&crd); err != nil {
				break // io.EOF, or a document that is not YAML: no more CRDs here
			}
			if crd.Kind != "CustomResourceDefinition" {
				continue
			}
			out.Scopes[schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}] = crd.Spec.Scope != apiextensionsv1.ClusterScoped
			for _, v := range crd.Spec.Versions {
				if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
					continue
				}
				internal := &apiextensions.JSONSchemaProps{}
				if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v.Schema.OpenAPIV3Schema, internal, nil); err != nil {
					continue
				}
				structural, err := structuralschema.NewStructural(internal)
				if err != nil {
					continue
				}
				gvk := schema.GroupVersionKind{Group: crd.Spec.Group, Version: v.Name, Kind: crd.Spec.Names.Kind}
				out.Defaults[gvk] = func(obj map[string]interface{}) { structuraldefaulting.Default(obj, structural) }
			}
		}
	}
	return out
}

// missing says what the in-memory run cannot do for gvk for want of its CRD
// file, as a note for the result; "" when the file was read and has a
// structural schema for gvk's version.
func (c crdFacts) missing(gvk schema.GroupVersionKind) string {
	if _, ok := c.Scopes[gvk.GroupKind()]; !ok {
		return fmt.Sprintf("no CRD file for %s — schema defaults and scope not applied; set crdFile in the Blueprint or spec.crd", gvk.Kind)
	}
	if _, ok := c.Defaults[gvk]; !ok {
		return fmt.Sprintf("the CRD file for %s has no structural schema for %s — schema defaults not applied", gvk.Kind, gvk.Version)
	}
	return ""
}
