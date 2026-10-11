// pkg/runners/clusterrolebindings.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/clusterrolebindings"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunClusterRoleBindings resolves and applies ClusterRoleBinding template declarations.
//
// ClusterRoleBindings are cluster-scoped — the namespace guard is not applied.
// Ownership is tracked via the inrun.dev/owner label; auto-GC via
// OwnerReferences is not possible for cluster-scoped resources.
func RunClusterRoleBindings(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ClusterRoleBindingTemplateSource,
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
					if err := clusterrolebindings.DeleteIfOwned(ctx, kube, owner, name); err != nil {
						return fmt.Errorf("clusterRoleBindings[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "ClusterRoleBinding").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resolver.ResolveClusterRoleBindingTemplate(src)
		if err != nil {
			return fmt.Errorf("clusterRoleBindings[%d]: %w", i, err)
		}

		spec := clusterrolebindings.Resolve(resolved, resolver.OwnerName())

		if update {
			if err := clusterrolebindings.Update(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("clusterRoleBindings[%d].update: %w", i, err)
			}
		} else {
			if err := clusterrolebindings.Create(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("clusterRoleBindings[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := clusterrolebindings.Update(ctx, kube, owner, spec); err != nil {
					return fmt.Errorf("clusterRoleBindings[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
