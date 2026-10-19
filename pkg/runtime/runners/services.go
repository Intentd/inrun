// pkg/runners/services.go
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
	"github.com/intentd/kuberesources/core/services"
)

// RunServices resolves and applies Service template declarations.
// Same update/reconcile: true semantics as RunDeployments.
func RunServices(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ServiceTemplateSource,
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
		// 1. Evaluate conditions BEFORE resolving templates
		conditionPassed := types.EvaluateConditions(resolver.Data(), src.Conditions, src.Or, resolver.TemplateEvaluator())

		name, _ := resolver.Resolve(src.Name)
		ns, _ := resolver.Resolve(src.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}

		// ── Namespace guard ───────────────────────────────────────────────
		// Called after namespace resolution so the actual target namespace
		// is known. Before this point the namespace may be a template expression.
		if guard != nil && !guard(ctx, owner, ns) {
			continue // skipped — CheckNamespace already logged the reason
		}

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					res, err := services.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("services[%d]: conditional cleanup: %w", i, err)
					}
					logResult("service", name, ns, owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "ConfigMap").
				Int("index", i).
				Msg("conditions not met — skipping resource")

			continue
		}

		// 2. Resolve template expressions
		resolved, err := resources.ResolveService(resolver, src)
		if err != nil {
			return fmt.Errorf("services[%d]: %w", i, err)
		}

		// 3. Build registry spec and apply
		spec := resources.ServiceSpec(resolved, owner)

		if update {
			res, err := services.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("services[%d].update: %w", i, err)
			}
			logResult("service", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := services.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("services[%d].create: %w", i, err)
			}
			logResult("service", spec.Name, spec.Namespace, owner, res)

			// reconcile: true
			if src.Reconcile {
				res, err := services.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("services[%d].reconcile: %w", i, err)
				}
				logResult("service", spec.Name, spec.Namespace, owner, res)
			}
		}
	}
	return nil
}
