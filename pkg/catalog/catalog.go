package catalog

import (
	"fmt"
	"reflect"

	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/typeregistry"
	"github.com/inrundev/inrun/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// -----------------------------------------------------------------------------
// Entry point
// -----------------------------------------------------------------------------

// NewSchemeRegistry returns a new scheme
func NewSchemeRegistry(k *Catalog) (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	// 1. Register built-in Kubernetes types
	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)

	// 2. Register core Kubernetes types
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// 3. Register CRDs
	var err error
	// Register dynamic scheme
	if scheme, err = k.registerDynamicScheme(scheme); err != nil {
		return nil, err
	}
	// Register typed scheme. The gateway handles every CR as unstructured
	// (UseUnstructured), so a typed registration would clash with it.
	if k.config == nil || !k.config.IsGatewayInstance() {
		if scheme, err = typeregistry.RegisterTypedScheme(scheme); err != nil {
			return nil, err
		}
	}

	// 4. Register external GVKs from custom: blocks so the fake dynamic client's
	// object tracker can create/get them during simulate (in-memory cluster).
	// Without this, the tracker has no schema entry and Create calls fail silently.
	if scheme, err = k.registerCustomResourceScheme(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}

// UseUnstructured switches every CRD to dynamic mode, so each CR is handled
// as *unstructured.Unstructured and no compiled type is needed. The gateway
// calls it: it serves, admits and views CRs but never reconciles them.
func (k *Catalog) UseUnstructured() {
	for name, crd := range k.enabledCRDs {
		k.enabledCRDs[name] = unstructuredEntry(crd)
	}
	for name, crd := range k.Spec.CRDs {
		k.Spec.CRDs[name] = unstructuredEntry(crd)
	}
}

func unstructuredEntry(crd types.CRDEntry) types.CRDEntry {
	if crd.IsBuiltInType() {
		return crd
	}
	crd.Mode = types.CRDModeDynamic
	crd.APITypes.Location = ""
	return crd
}

// Helpers
// UpdateResourceMapAndReturn maps each enabled CRD's dynamic object type into
// the package-level resourceTypeMap for O(1) GVK reverse-lookups, then returns k.
func (k *Catalog) UpdateResourceMapAndReturn() (*Catalog, error) {
	// Map the type of the object
	for _, c := range k.enabledCRDs {
		if k.EnabledCRDsEmpty() {
			return nil, fmt.Errorf("no enabled CRDs found")
		}

		// Map the type of the object
		logger.Debug().Msgf("updating resource map for %s", c.GroupVersionKind.String())
		resourceTypeMap[reflect.TypeOf(c.DynamicModeObject)] = c.GroupVersionKind.String()
	}

	return k, nil
}

// registerCustomResourceScheme registers external GVKs declared in custom: blocks
// (onCreate and onReconcile) into the scheme so the fake dynamic client's object
// tracker can store and retrieve them during simulate. Deduplicates by GVK string.
func (k *Catalog) registerCustomResourceScheme(scheme *runtime.Scheme) (*runtime.Scheme, error) {
	seen := make(map[string]bool)
	for _, crd := range k.enabledCRDs {
		if crd.HasOnCreate() && crd.Box().EffectiveOnCreate().CustomResource != nil {
			for i := range crd.Box().EffectiveOnCreate().CustomResource {
				cr := &crd.Box().EffectiveOnCreate().CustomResource[i]
				key := cr.APIVersion + "/" + cr.Kind
				if seen[key] || cr.APIVersion == "" || cr.Kind == "" {
					continue
				}
				seen[key] = true
				gvk, err := cr.BuildGVK()
				if err != nil {
					continue
				}
				scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"}, &unstructured.UnstructuredList{})
			}
		}

		if crd.HasOnReconcile() && crd.Box().EffectiveOnReconcile().CustomResource != nil {
			for i := range crd.Box().EffectiveOnReconcile().CustomResource {
				cr := &crd.Box().EffectiveOnReconcile().CustomResource[i]
				key := cr.APIVersion + "/" + cr.Kind
				if seen[key] || cr.APIVersion == "" || cr.Kind == "" {
					continue
				}
				seen[key] = true
				gvk, err := cr.BuildGVK()
				if err != nil {
					continue
				}
				scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"}, &unstructured.UnstructuredList{})
			}
		}

	}
	return scheme, nil
}

// Register dynamic CRDs — tells the watch stream to decode
// these GVKs as *unstructured.Unstructured instead of failing
func (k *Catalog) registerDynamicScheme(scheme *runtime.Scheme) (*runtime.Scheme, error) {
	for _, crd := range k.enabledCRDs {
		if crd.IsDynamic() && crd.APITypes.Location == "" && !crd.IsBuiltInType() {
			// Register Object
			scheme.AddKnownTypeWithName(
				schema.GroupVersionKind{
					Group:   crd.APITypes.Group,
					Version: crd.APITypes.Version,
					Kind:    crd.APITypes.Kind,
				},
				&unstructured.Unstructured{},
			)

			// Register List
			scheme.AddKnownTypeWithName(
				schema.GroupVersionKind{
					Group:   crd.APITypes.Group,
					Version: crd.APITypes.Version,
					Kind:    crd.APITypes.Kind + "List",
				},
				&unstructured.UnstructuredList{},
			)
		}
	}

	return scheme, nil
}
