package generic

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/customresources"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// runCustomResources resolves and applies CustomResource template declarations.
func runCustomResources(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.CustomResourceTemplateSource,
	update bool,
	guard func(ctx context.Context, obj domain.Object, ns string) bool,
	labelMgr *labels.Manager,
	shouldProtect bool,
) error {
	// Track active names for conditional cleanup when resources are no longer desired.
	activeNames := make(map[string]bool, len(srcs))
	for _, s := range srcs {
		if !types.EvaluateConditions(resolver.Data(), s.Conditions, s.Or, resolver.TemplateEvaluator()) {
			continue
		}
		n, _ := resolver.Resolve(s.Metadata.Name)
		nsp, _ := resolver.Resolve(s.Metadata.Namespace)
		if nsp == "" {
			nsp = owner.GetNamespace()
		}
		activeNames[nsp+"/"+n] = true
	}

	for i, src := range srcs {
		conditionPassed := types.EvaluateConditions(resolver.Data(), src.Conditions, src.Or, resolver.TemplateEvaluator())

		// Resolve name/namespace for guard/cleanup decisions
		name, _ := resolver.Resolve(src.Metadata.Name)
		ns, _ := resolver.Resolve(src.Metadata.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}

		// Optional guard (e.g., cluster-scoped guard or admission)
		if guard != nil && !guard(ctx, owner, ns) {
			continue
		}

		// If condition not met, optionally cleanup previously created resource
		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					if err := customresources.DeleteIfOwned(ctx, kube.DynamicClient(), kube.RESTMapper(), owner, name, ns, src.APIVersion, src.Kind); err != nil {
						return fmt.Errorf("custom[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "CustomResource").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		// Check that the target CRD exists before resolving the template.
		// If the CRD is not yet installed, the mapper will return a "no matches"
		// error. We skip gracefully — the retryMissingCRDs loop will log when it
		// appears and refresh the mapper.
		if gvk, gvkErr := src.BuildGVK(); gvkErr == nil {
			if _, mapErr := src.ResolveGVR(kube.RESTMapper()); mapErr != nil {
				logger.FromContext(ctx).Warn().
					Str("gvk", gvk.String()).
					Msgf("custom[%d]: CRD not yet available '%s' — skipping until it appears", i, gvk.String())
				continue
			}
		}

		// Resolve the templated custom resource declaration into a concrete CustomResource
		resolved, err := resources.ResolveCustomResource(resolver, src)
		if err != nil {
			return fmt.Errorf("custom[%d]: resolve template: %w", i, err)
		}

		// Convert resolved template into the runtime customresources.Spec
		spec := resources.CustomResourceSpec(resolved, resolver.OwnerName())

		// Create is a no-op if the resource already exists (idempotent OnCreate).
		// Update always corrects drift — delete drift (recreate) and spec drift.
		if update {
			if err := customresources.Apply(ctx, kube.DynamicClient(), kube.RESTMapper(), owner, spec, labelMgr, shouldProtect); err != nil {
				return fmt.Errorf("custom[%d].update: %w", i, err)
			}
		} else {
			if err := customresources.Create(ctx, kube.DynamicClient(), kube.RESTMapper(), owner, spec, labelMgr, shouldProtect); err != nil {
				return fmt.Errorf("custom[%d].create: %w", i, err)
			}

			// reconcile: true
			if src.Reconcile {
				if err := customresources.Apply(ctx, kube.DynamicClient(), kube.RESTMapper(), owner, spec, labelMgr, shouldProtect); err != nil {
					return fmt.Errorf("custom[%d].reconcile: %w", i, err)
				}
			}
		}
	}

	return nil
}
