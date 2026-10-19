// pkg/runners/pvs.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/pvs"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunPVs resolves and applies PersistentVolume template declarations.
// PVs are cluster-scoped; the namespace guard is intentionally not applied.
func RunPVs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PVTemplateSource,
	update bool,
) error {
	activeNames := make(map[string]bool, len(srcs))
	for _, s := range srcs {
		if !types.EvaluateConditions(resolver.Data(), s.Conditions, s.Or, resolver.TemplateEvaluator()) {
			continue
		}
		n, _ := resolver.Resolve(s.Name)
		activeNames[n] = true
	}

	for i, src := range srcs {
		conditionPassed := types.EvaluateConditions(resolver.Data(), src.Conditions, src.Or, resolver.TemplateEvaluator())

		name, _ := resolver.Resolve(src.Name)

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[name] {
					if err := pvs.DeleteIfOwned(ctx, kube.Clientset(), owner, name); err != nil {
						return fmt.Errorf("pvs[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "PersistentVolume").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolvePV(resolver, src)
		if err != nil {
			return fmt.Errorf("pvs[%d]: %w", i, err)
		}

		spec := resources.PVSpec(resolved, resolver.OwnerName())

		if update {
			if err := pvs.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
				return fmt.Errorf("pvs[%d].update: %w", i, err)
			}
		} else {
			if err := pvs.Create(ctx, kube.Clientset(), owner, spec); err != nil {
				return fmt.Errorf("pvs[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := pvs.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
					return fmt.Errorf("pvs[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
