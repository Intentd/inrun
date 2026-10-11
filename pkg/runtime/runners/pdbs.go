// pkg/runners/pdbs.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/pdbs"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunPDBs resolves and applies PodDisruptionBudget template declarations.
func RunPDBs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PDBTemplateSource,
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
					if err := pdbs.DeleteIfOwned(ctx, kube, owner, name, ns); err != nil {
						return fmt.Errorf("pdbs[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "PodDisruptionBudget").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resolver.ResolvePDBTemplate(src)
		if err != nil {
			return fmt.Errorf("pdbs[%d]: %w", i, err)
		}

		spec := pdbs.Resolve(resolved, resolver.OwnerName(), resolver.Profiles())

		if update {
			if err := pdbs.Update(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("pdbs[%d].update: %w", i, err)
			}
		} else {
			if err := pdbs.Create(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("pdbs[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := pdbs.Update(ctx, kube, owner, spec); err != nil {
					return fmt.Errorf("pdbs[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
