package reconcile

import (
	"fmt"
	"strings"
	texttemplate "text/template"

	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/kube"
)

// validateCustomResources validates any Custom resources declared under the
// OperatorBox hooks (onCreate/onReconcile/onDelete).
// This function delegates per-item structural checks to types.validateCustomResource
// which enforces apiVersion/kind/metadata/name/namespace/labels/annotations/template
// rules and the namespaced defaulting semantics.
func (c *checker) validateCustomResources() error {
	for name, crd := range c.Blueprint.EnabledCRDs() {
		// Fast path: nothing to validate if constructor is declared (reconciler.default: false).
		if crd.WithConstructorDecl() {
			logger.Debug().
				Str("crd", name).
				Msg("skipping custom resource validation: constructor declared")
			continue
		}

		if !crd.HasAnyHookTemplates() {
			logger.Debug().
				Str("crd", name).
				Msg("skipping custom resource validation: no hook templates")
			continue
		}

		// Fast path: if the CRD doesn't declare any custom resources, skip.
		if !crd.HasAnyCustomResources() {
			logger.Debug().
				Str("crd", name).
				Msg("no custom resources declared in onCreate/onReconcile; skipping validation")
			continue
		}

		// Validate OnCreate custom resources
		if crd.HasOnCreate() && crd.Box().EffectiveOnCreate().CustomResource != nil {
			for i, cr := range crd.Box().EffectiveOnCreate().CustomResource {
				path := fmt.Sprintf("%s.onCreate.custom[%d]", name, i)
				// Delegate structural validation to the types package.
				if err := ValidateCustomResource(&crd, &cr, path, buildFuncMapForValidation(c.Blueprint.Functions)); err != nil {
					return err
				}

				// Additional blueprint-level checks (defensive):
				// - If the declaration explicitly marks cluster-scoped but provided a namespace, error.
				if cr.Metadata.Namespaced != nil && !*cr.Metadata.Namespaced && cr.Metadata.Namespace != "" {
					return fmt.Errorf("%s %s: metadata.namespaced=false but metadata.namespace is set (cluster-scoped resources must not set namespace)", failureMark(), path)
				}

				// Log a debug line for observability
				logger.Debug().
					Str("crd", name).
					Str("path", path).
					Msg("validated onCreate custom resource declaration")
			}
		}

		// Validate OnReconcile custom resources
		if crd.HasOnReconcile() && crd.Box().EffectiveOnReconcile().CustomResource != nil {
			for i, cr := range crd.Box().EffectiveOnReconcile().CustomResource {
				path := fmt.Sprintf("%s.onReconcile.custom[%d]", name, i)
				if err := ValidateCustomResource(&crd, &cr, path, buildFuncMapForValidation(c.Blueprint.Functions)); err != nil {
					return err
				}

				if cr.Metadata.Namespaced != nil && !*cr.Metadata.Namespaced && cr.Metadata.Namespace != "" {
					return fmt.Errorf("%s %s: metadata.namespaced=false but metadata.namespace is set (cluster-scoped resources must not set namespace)", failureMark(), path)
				}

				logger.Debug().
					Str("crd", name).
					Str("path", path).
					Msg("validated onReconcile custom resource declaration")
			}
		}
	}

	return nil
}

// ValidateCustomResource validates a single CustomResource declaration. It
// enforces the structural and semantic rules required by Inrun, and checks
// spec against the child's CRD schema when the declaration names a crdFile.
//
// Validation responsibilities:
//   - Ensure required top-level fields (apiVersion, kind) are present and well-formed.
//   - Ensure metadata.name is present after templating.
//   - Enforce namespaced vs cluster-scoped semantics using Metadata.Namespaced
//     with a defensive default of namespaced=true.
//   - Validate labels and annotations.
//   - With crdFile, check spec against the child kind's CRD schema.
//   - Return path-aware errors so callers can point users to the exact declaration.
//
// funcMap is what templated values are parsed against: the built-in functions
// and the Blueprint's own. nil means the built-ins only.
func ValidateCustomResource(crd *types.CRDEntry, cr *types.CustomResourceTemplateSource, path string, funcMap texttemplate.FuncMap) error {
	if funcMap == nil {
		funcMap = buildFuncMapForValidation(types.KELFunctionRegistry{})
	}
	if cr == nil {
		return fmt.Errorf("%s %s: custom resource declaration is nil", failureMark(), path)
	}

	// --- apiVersion ---------------------------------------------------------
	if strings.TrimSpace(cr.APIVersion) == "" {
		return fmt.Errorf("%s %s: missing required field 'apiVersion'", failureMark(), path)
	}
	if !strings.Contains(cr.APIVersion, "/") {
		return fmt.Errorf("%s %s: apiVersion %q is invalid — must be group/version (e.g. foo.io/v1)", failureMark(), path, cr.APIVersion)
	}

	// --- native type guard --------------------------------------------------
	// Reject built-in Kubernetes types that Inrun manages natively via
	// HookTemplates. Using custom: for these panics during simulate (scheme
	// double-registration) and bypasses native features like drift correction
	// and presets. The builtInRegistry is the single source of truth.
	gvk, err := kube.GVKFromFields(cr.APIVersion, cr.Kind)
	if err != nil {
		return fmt.Errorf("%s %s: %w", failureMark(), path, err)
	}
	if b, isNative := children.LookupBuiltInByGVK(gvk.Group, gvk.Version, cr.Kind); isNative && b.IsChild && b.HookKey != "" {
		return fmt.Errorf(
			"%s %s: %s %s is a native Inrun resource — use %s: instead of custom:",
			failureMark(), path, cr.APIVersion, cr.Kind, b.HookKey,
		)
	}

	// --- kind ---------------------------------------------------------------
	if strings.TrimSpace(cr.Kind) == "" {
		return fmt.Errorf("%s %s: missing required field 'kind'", failureMark(), path)
	}

	// --- metadata -----------------------------------------------------------
	// Defensive: metadata must be present and name required after templating.
	name := cr.Metadata.Name
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s %s: metadata.name is required", failureMark(), path)
	}
	if isTemplate(name) {
		// The text around the expressions is in every name it can produce.
		if err := validateTemplate("custom", path, "metadata.name", "name", name, funcMap); err != nil {
			return err
		}
		if err := isValidK8sName(templateSample(name)); err != nil {
			return fmt.Errorf("%s %s: metadata.name %q can never be a valid name: %w", failureMark(), path, name, err)
		}
	} else if err := isValidK8sName(name); err != nil {
		return fmt.Errorf("%s metadata.name: %w", failureMark(), err)
	}

	// --- metadata.labels/annotations -----------------------------------------------------------
	// Validate that the key is a valid Kubernetes label/annotation key
	labels := cr.Metadata.Labels
	annotations := cr.Metadata.Annotations
	for k, v := range labels {
		if errs := isValidLabelKey(k); len(errs) > 0 {
			return fmt.Errorf("%s %s: metadata.labels is not a valid label key", failureMark(), k)
		}
		if isTemplate(v) {
			if err := validateTemplate("custom", path, "metadata.labels."+k, "value", v, funcMap); err != nil {
				return err
			}
			if errs := isValidLabelValue(templateSample(v)); len(errs) > 0 {
				return fmt.Errorf("%s %s: metadata.labels.%s %q can never be a valid label value: %s", failureMark(), path, k, v, strings.Join(errs, "; "))
			}
			continue
		}
		if errs := isValidLabelValue(v); len(errs) > 0 {
			return fmt.Errorf("%s %s: metadata.labels is not a valid label value", failureMark(), k)
		}
	}
	for k := range annotations {
		if errs := isValidLabelKey(k); len(errs) > 0 {
			return fmt.Errorf("%s %s: metadata.annotations is not a valid annotation key", failureMark(), k)
		}
	}
	// Namespaced defaulting: nil => true (namespaced by default)
	namespaced := true
	if cr.Metadata.Namespaced != nil {
		namespaced = *cr.Metadata.Namespaced
	}

	// If the declaration explicitly marks cluster-scoped but provided a namespace, error.
	if !namespaced && cr.Metadata.Namespace != "" {
		return fmt.Errorf("%s %s: metadata.namespaced=false but metadata.namespace is set (cluster-scoped resources must not set namespace)", failureMark(), path)
	}

	// If namespaced (default) then namespace must be present (non-Empty().
	// Note: We allow empty namespace when the reconciler will template it to a value,
	// but at validation time we require the field to be present (non-Empty() to avoid
	// accidental cluster-scoped creations. If you prefer to allow templated empty
	// namespaces, relax this check accordingly.
	if namespaced && strings.TrimSpace(cr.Metadata.Namespace) == "" {
		return fmt.Errorf("%s %s: metadata.namespace is required for namespaced custom resources (metadata.namespaced is true or unspecified)", failureMark(), path)
	}

	// --- spec against the child's CRD ---------------------------------------
	if cr.ResolvedCRDFile != "" && len(cr.Spec) > 0 {
		// A key holding a template expression is only known at reconcile.
		unknown, err := kube.UnknownSpecFields(cr.ResolvedCRDFile, cr.Spec, isTemplate)
		if err != nil {
			return fmt.Errorf("%s %s: crdFile: %w", failureMark(), path, err)
		}
		if len(unknown) > 0 {
			return fmt.Errorf("%s %s: not in %s's schema, so the API server would drop or reject them: %s",
				failureMark(), path, cr.Kind, strings.Join(unknown, ", "))
		}
	}

	return nil
}
