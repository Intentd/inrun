// pkg/runners/crs.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	crs "github.com/intentd/kubewright/core/clusterroles"
)

// RunClusterRoles resolves and applies ClusterRole template declarations.
//
// ClusterRoles are cluster-scoped — the namespace guard is not applied.
// Ownership is tracked via the inrun.dev/owner label; auto-GC via
// OwnerReferences is not possible for cluster-scoped resources.
func RunClusterRoles(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ClusterRoleTemplateSource,
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
					res, err := crs.DeleteIfOwned(ctx, kube.Clientset(), name, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("clusterRoles[%d]: conditional cleanup: %w", i, err)
					}
					logResult("clusterrole", name, "", owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "ClusterRole").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveClusterRole(resolver, src)
		if err != nil {
			return fmt.Errorf("clusterRoles[%d]: %w", i, err)
		}

		spec := resources.ClusterRoleSpec(resolved, owner)

		if update {
			res, err := crs.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("clusterRoles[%d].update: %w", i, err)
			}
			logResult("clusterrole", spec.Name, "", owner, res)
		} else {
			res, err := crs.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("clusterRoles[%d].create: %w", i, err)
			}
			logResult("clusterrole", spec.Name, "", owner, res)
			if src.Reconcile {
				res, err := crs.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("clusterRoles[%d].reconcile: %w", i, err)
				}
				logResult("clusterrole", spec.Name, "", owner, res)
			}
		}
	}
	return nil
}
