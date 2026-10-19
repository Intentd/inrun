package blueprint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/presets"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// SetGroupVersionKind resolves GroupVersionKind, GroupVersionResource, and
// GroupVersion on each enabled CRD entry, then rebuilds lookup indexes.
// Must be called after BuildRuntimeBlueprint and before validators.
func (c *Blueprint) SetGroupVersionKind() error {
	for name, crd := range c.enabledCRDs {
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
		c.enabledCRDs[name] = crd
	}
	c.BuildLookupIndexes()
	return nil
}

// SetDefaults applies configuration defaults to every enabled CRD entry:
// naming, namespacing, reconciler settings, finalizers, and notifications.
// Must be called after SetGroupVersionKind.
func (c *Blueprint) SetDefaults(cfg *config.Config) error {
	for name, crd := range c.enabledCRDs {
		metaN := c.metadata.Name
		if metaN != "" {
			if errs := validation.IsDNS1123Label(metaN); len(errs) > 0 {
				return fmt.Errorf("%s CRD with key '%s': invalid metadata.Name %q: %s", failureMark(), name, metaN, strings.Join(errs, "; "))
			}
			crd.BlueprintName = metaN
		} else {
			crd.BlueprintName = c.ClusterName() + "-" + name
		}

		if crd.BlueprintNamespace == "" {
			metaNs := c.metadata.Namespace
			if metaNs != "" {
				if errs := validation.IsDNS1123Label(metaNs); len(errs) > 0 {
					return fmt.Errorf("%s CRD with key '%s': invalid metadata.Namespace %q: %s", failureMark(), name, metaNs, strings.Join(errs, "; "))
				}
				crd.BlueprintNamespace = metaNs
			} else {
				crd.BlueprintNamespace = "default"
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
		boxFinalizers = append(boxFinalizers, c.Spec.Finalizers...)
		if crd.HasServeTarget() {
			for _, target := range crd.Serve.Target.Entries {
				if target.OperatorBox.Empty() {
					continue
				}
				boxFinalizers = append(boxFinalizers, target.OperatorBox.EffectiveFinalizers()...)
			}
		}
		// Every CR Inrun manages carries its cleanup finalizer, whatever the
		// reconciler: deletion waits for a finalizing reconcile to finish.
		if !slices.Contains(boxFinalizers, labels.CleanupFinalizer) {
			boxFinalizers = append(boxFinalizers, labels.CleanupFinalizer)
		}
		if crd.Box().Runtime == nil {
			crd.Box().Runtime = &types.RuntimeConfig{}
		}
		crd.Box().Runtime.SetFinalizerNames(boxFinalizers)

		if crd.Box().Reconcile == nil {
			crd.Box().Reconcile = &types.ReconcileConfig{}
		}
		rec := crd.Box().Reconcile
		if rec.Preset != "" {
			result, err := presets.ApplyReconcilerPreset(rec.Preset, &c.Presets)
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
			rec.Workers = cfg.Blueprint().DefaultWorkers()
		}
		if rec.Resync.Duration == 0 {
			rec.Resync.Duration = cfg.Blueprint().DefaultResync()
		}
		if rec.Queue.FailureThreshold == 0 {
			rec.Queue.FailureThreshold = cfg.Blueprint().DefaultFailureThreshold()
		}
		crd.Box().Reconcile = rec

		c.enabledCRDs[name] = crd
	}
	return nil
}
