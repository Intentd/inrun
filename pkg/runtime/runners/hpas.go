// pkg/runners/hpas.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/hpas"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunHPAs resolves and applies HorizontalPodAutoscaler template declarations.
func RunHPAs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.HPATemplateSource,
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
					if err := hpas.DeleteIfOwned(ctx, kube, owner, name, ns); err != nil {
						return fmt.Errorf("hpas[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "HorizontalPodAutoscaler").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resolver.ResolveHPATemplate(src)
		if err != nil {
			return fmt.Errorf("hpas[%d]: %w", i, err)
		}

		spec := hpas.Resolve(resolved, resolver.OwnerName(), resolver.Profiles())

		if update {
			if err := hpas.Update(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("hpas[%d].update: %w", i, err)
			}
		} else {
			if err := hpas.Create(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("hpas[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := hpas.Update(ctx, kube, owner, spec); err != nil {
					return fmt.Errorf("hpas[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
