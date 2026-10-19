// pkg/runners/resourcequotas.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/resourcequotas"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunResourceQuotas resolves and applies ResourceQuota template declarations.
//
// resourceQuotas support fromResourceQuota to copy an existing quota's hard limits,
// and toNamespaces to distribute copies across multiple namespaces.
//
// reconcile: true — re-reads the source quota on every reconcile and syncs changes.
func RunResourceQuotas(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ResourceQuotaTemplateSource,
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
					if err := resourcequotas.DeleteIfOwned(ctx, kube, owner, name, ns); err != nil {
						return fmt.Errorf("resourceQuotas[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "ResourceQuota").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveResourceQuota(resolver, src)
		if err != nil {
			return fmt.Errorf("resourceQuotas[%d]: %w", i, err)
		}

		spec := resources.ResourceQuotaSpec(resolved, resolver.OwnerName(), resolver.Presets())

		if len(resolved.ToNamespaces) > 0 {
			namespaces, err := resolver.ResolveStringSlice(resolved.ToNamespaces)
			if err != nil {
				return fmt.Errorf("resourceQuotas[%d].toNamespaces: %w", i, err)
			}

			shouldSync := update || src.Reconcile
			for _, targetNs := range namespaces {
				if guard != nil && !guard(ctx, owner, targetNs) {
					continue
				}
				nsSpec := spec
				nsSpec.Namespace = targetNs
				if shouldSync {
					if err := resourcequotas.Apply(ctx, kube, owner, nsSpec); err != nil {
						return fmt.Errorf("resourceQuotas[%d].update namespace=%s: %w", i, targetNs, err)
					}
				} else {
					if err := resourcequotas.Create(ctx, kube, owner, nsSpec); err != nil {
						return fmt.Errorf("resourceQuotas[%d].create namespace=%s: %w", i, targetNs, err)
					}
				}
			}
			continue
		}

		if update {
			if err := resourcequotas.Apply(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("resourceQuotas[%d].update: %w", i, err)
			}
		} else {
			if err := resourcequotas.Create(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("resourceQuotas[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := resourcequotas.Apply(ctx, kube, owner, spec); err != nil {
					return fmt.Errorf("resourceQuotas[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
