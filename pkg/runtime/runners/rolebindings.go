// pkg/runners/rolebindings.go
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
	"github.com/intentd/kuberesources/core/rolebindings"
)

// RunRoleBindings resolves and applies RoleBinding template declarations.
//
// On onCreate: idempotent create only.
// On onReconcile (update=true): creates or updates (or recreates when roleRef changed).
// Owner references ensure cleanup when the CR is deleted.
func RunRoleBindings(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.RoleBindingTemplateSource,
	update bool,
	guard func(ctx context.Context, obj domain.Object, ns string) bool,
) error {
	activeNames := make(map[string]bool, len(srcs))
	for _, s := range srcs {
		if !types.EvaluateConditions(resolver.Data(), s.Conditions, s.Or, resolver.TemplateEvaluator()) {
			continue
		}
		n, _ := resolver.Resolve(s.Name)
		nsp, _ := resolver.Resolve(s.Namespace)
		if nsp == "" {
			nsp = owner.GetNamespace()
		}
		activeNames[nsp+"/"+n] = true
	}

	for i, src := range srcs {
		conditionPassed := types.EvaluateConditions(resolver.Data(), src.Conditions, src.Or, resolver.TemplateEvaluator())

		name, _ := resolver.Resolve(src.Name)
		ns, _ := resolver.Resolve(src.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}

		if guard != nil && !guard(ctx, owner, ns) {
			continue
		}

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					res, err := rolebindings.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("roleBindings[%d]: conditional cleanup: %w", i, err)
					}
					logResult("rolebinding", name, ns, owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "RoleBinding").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveRoleBinding(resolver, src)
		if err != nil {
			return fmt.Errorf("roleBindings[%d]: %w", i, err)
		}

		spec := resources.RoleBindingSpec(resolved, owner)

		if update || src.Reconcile {
			res, err := rolebindings.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("roleBindings[%d].update: %w", i, err)
			}
			logResult("rolebinding", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := rolebindings.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("roleBindings[%d].create: %w", i, err)
			}
			logResult("rolebinding", spec.Name, spec.Namespace, owner, res)
		}
	}
	return nil
}
