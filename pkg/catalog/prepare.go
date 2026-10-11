package catalog

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// SetGroupVersionKind resolves GroupVersionKind, GroupVersionResource, and
// GroupVersion on each enabled CRD entry, then rebuilds lookup indexes.
// Must be called after BuildRuntimeCatalog and before validators.
func (k *Catalog) SetGroupVersionKind() error {
	for name, crd := range k.enabledCRDs {
		crd.GroupVersionKind = schema.GroupVersionKind{
			Group:   crd.APITypes.Group,
			Version: crd.APITypes.Version,
			Kind:    crd.APITypes.Kind,
		}
		crd.GroupVersionResource = schema.GroupVersionResource{
			Group:    crd.APITypes.Group,
			Version:  crd.APITypes.Version,
			Resource: crd.APITypes.Plural,
		}
		crd.GroupVersion = &schema.GroupVersion{
			Group:   crd.APITypes.Group,
			Version: crd.APITypes.Version,
		}

		if crd.GroupVersionKind.Empty() {
			if crd.CRDFile != "" {
				return fmt.Errorf("CRD '%s': could not determine group/version/kind from crdFile %q", name, crd.CRDFile)
			}
			return fmt.Errorf("CRD '%s': missing required fields: apiTypes.group, apiTypes.version, apiTypes.kind (or declare crdFile: to read these from the CRD YAML)", name)
		}
		if crd.GroupVersion.Empty() {
			return fmt.Errorf("CRD '%s': missing required fields: apiTypes.group, apiTypes.version", name)
		}
		if crd.GroupVersionResource.Empty() {
			return fmt.Errorf("CRD '%s': missing required fields: apiTypes.plural", name)
		}
		if crd.Description == "" {
			crd.Description = fmt.Sprintf("%s CRD, GVK: %s", crd.APITypes.Kind, crd.GroupVersionKind.String())
		}
		k.enabledCRDs[name] = crd
	}
	k.BuildLookupIndexes()
	return nil
}

// SetDefaults applies configuration defaults to every enabled CRD entry:
// naming, namespacing, reconciler settings, finalizers, and notifications.
// Must be called after SetGroupVersionKind.
func (k *Catalog) SetDefaults(kfg *config.Config) error {
	for name, crd := range k.enabledCRDs {
		metaN := k.metadata.Name
		if metaN != "" {
			if errs := validation.IsDNS1123Label(metaN); len(errs) > 0 {
				return fmt.Errorf("%s CRD with key '%s': invalid metadata.Name %q: %s", failureMark(), name, metaN, strings.Join(errs, "; "))
			}
			crd.CatalogName = metaN
		} else {
			crd.CatalogName = k.ClusterName() + "-" + name
		}

		if crd.CatalogNamespace == "" {
			metaNs := k.metadata.Namespace
			if metaNs != "" {
				if errs := validation.IsDNS1123Label(metaNs); len(errs) > 0 {
					return fmt.Errorf("%s CRD with key '%s': invalid metadata.Namespace %q: %s", failureMark(), name, metaNs, strings.Join(errs, "; "))
				}
				crd.CatalogNamespace = metaNs
			} else {
				crd.CatalogNamespace = "default"
			}
		}

		crd.Name = name
		crd.Name = strings.ReplaceAll(crd.Name, " ", "")
		crd.Name = strings.ToLower(crd.Name)
		if crd.Name == "" {
			return fmt.Errorf("%s CRD with key '%s': empty name after normalisation", failureMark(), name)
		}
		if errs := validation.IsDNS1123Label(crd.Name); len(errs) > 0 {
			return fmt.Errorf("%s CRD with key '%s': invalid name %q: %s", failureMark(), name, crd.Name, strings.Join(errs, "; "))
		}

		if !crd.IsNamespaced() && crd.Namespace != "" {
			warning := fmt.Sprintf("%s is clusterscoped. Namespace %s will be ignored", crd.APITypes.Kind, crd.Namespace)
			crd.Warnings.AddWarning(warning)
			crd.Namespace = ""
		}
		if crd.IsNamespaced() && crd.Namespace != "" {
			if errs := validation.IsDNS1123Label(crd.Namespace); len(errs) > 0 {
				return fmt.Errorf("%s CRD with key '%s': invalid crd namespace %q: %s", failureMark(), name, crd.Namespace, strings.Join(errs, "; "))
			}
		}

		if crd.APITypes.APIPath == "" {
			logger.Debug().Msgf("API path for Kind=%s is empty. Setting to '/apis'", crd.APITypes.Kind)
			crd.APITypes.APIPath = "/apis"
		}
		if crd.APITypes.Plural == "" {
			logger.Debug().Msgf("Plural name for %s is empty. Setting to '%ss'", crd.APITypes.Kind, crd.Name)
			crd.APITypes.Plural = fmt.Sprintf("%ss", strings.ToLower(crd.APITypes.Kind))
		}

		boxFinalizers := crd.Box().EffectiveFinalizers()
		boxFinalizers = append(boxFinalizers, k.Spec.Finalizers...)
		if crd.HasServeTarget() {
			for _, target := range crd.Serve.Target.Entries {
				if target.OperatorBox.Empty() {
					continue
				}
				boxFinalizers = append(boxFinalizers, target.OperatorBox.EffectiveFinalizers()...)
			}
		}
		if crd.Box().Runtime == nil {
			crd.Box().Runtime = &types.RuntimeConfig{}
		}
		crd.Box().Runtime.Finalizers = boxFinalizers

		if crd.Box().Reconcile == nil {
			crd.Box().Reconcile = &types.ReconcileConfig{}
		}
		rec := crd.Box().Reconcile
		if rec.Profile != "" {
			result, err := profiles.ApplyReconcilerProfile(rec.Profile, &k.Profiles)
			if err != nil {
				return fmt.Errorf("%s CRD %q: %w", failureMark(), name, err)
			}
			if rec.Workers == 0 {
				rec.Workers = result.Workers
			}
			if rec.Resync.Duration == 0 {
				rec.Resync.Duration = result.Resync
			}
			if rec.Queue.MaxDepth == 0 {
				rec.Queue.MaxDepth = result.MaxDepth
			}
		}
		if rec.Workers == 0 {
			rec.Workers = kfg.Catalog().DefaultWorkers()
		}
		if rec.Resync.Duration == 0 {
			rec.Resync.Duration = kfg.Catalog().DefaultResync()
		}
		if rec.Queue.FailureThreshold == 0 {
			rec.Queue.FailureThreshold = kfg.Catalog().DefaultFailureThreshold()
		}
		crd.Box().Reconcile = rec

		k.enabledCRDs[name] = crd
	}
	return nil
}

// AddRuntimeObjects wires object and list constructor functions onto every
// enabled CRD entry. Dynamic CRDs get unstructured constructors; typed CRDs
// are looked up in the global ObjectRegistry / ListRegistry.
// Must be called after SetGroupVersionKind.
func (k *Catalog) AddRuntimeObjects() error {
	for name, crd := range k.enabledCRDs {
		gvk := crd.GroupVersionKind

		if crd.IsDynamic() {
			g := crd.APITypes.Group
			v := crd.APITypes.Version
			ki := crd.APITypes.Kind
			crd.DynamicModeObject = func() runtime.Object {
				u := &unstructured.Unstructured{}
				u.SetGroupVersionKind(schema.GroupVersionKind{Group: g, Version: v, Kind: ki})
				return u
			}
			crd.ListDynamicModeObject = func() runtime.Object {
				ul := &unstructured.UnstructuredList{}
				ul.SetGroupVersionKind(schema.GroupVersionKind{Group: g, Version: v, Kind: ki + "List"})
				return ul
			}
			k.enabledCRDs[name] = crd
			continue
		}

		objFn, ok := types.ObjectRegistry[gvk]
		if !ok {
			err := fmt.Errorf("addRuntimeObjects: no object constructor registered for %s", gvk)
			if crd.RegistryRef != "" {
				return &TypedOperatorError{Ref: crd.RegistryRef, Err: err}
			}
			return err
		}
		listFn, ok := types.ListRegistry[gvk]
		if !ok {
			err := fmt.Errorf("%s addRuntimeObjects: no list constructor registered for %s", failureMark(), gvk)
			if crd.RegistryRef != "" {
				return &TypedOperatorError{Ref: crd.RegistryRef, Err: err}
			}
			return err
		}
		crd.DynamicModeObject = objFn
		crd.ListDynamicModeObject = listFn
		k.enabledCRDs[name] = crd
	}
	return nil
}
