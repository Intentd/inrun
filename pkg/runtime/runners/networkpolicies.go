// pkg/runners/networkpolicies.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/networkpolicies"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunNetworkPolicies resolves and applies NetworkPolicy template declarations.
//
// networkPolicies support fromNetworkPolicy to copy an existing policy's spec,
// and toNamespaces to distribute copies across multiple namespaces.
//
// reconcile: true — re-reads the source policy on every reconcile and syncs changes.
func RunNetworkPolicies(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.NetworkPolicyTemplateSource,
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
					if err := networkpolicies.DeleteIfOwned(ctx, kube, owner, name, ns); err != nil {
						return fmt.Errorf("networkPolicies[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "NetworkPolicy").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resolver.ResolveNetworkPolicyTemplate(src)
		if err != nil {
			return fmt.Errorf("networkPolicies[%d]: %w", i, err)
		}

		spec := networkpolicies.Resolve(resolved, resolver.OwnerName(), resolver.Profiles())

		if len(resolved.ToNamespaces) > 0 {
			namespaces, err := resolver.ResolveStringSlice(resolved.ToNamespaces)
			if err != nil {
				return fmt.Errorf("networkPolicies[%d].toNamespaces: %w", i, err)
			}

			shouldSync := update || src.Reconcile
			for _, targetNs := range namespaces {
				if guard != nil && !guard(ctx, owner, targetNs) {
					continue
				}
				nsSpec := spec
				nsSpec.Namespace = targetNs
				if shouldSync {
					if err := networkpolicies.Update(ctx, kube, owner, nsSpec); err != nil {
						return fmt.Errorf("networkPolicies[%d].update namespace=%s: %w", i, targetNs, err)
					}
				} else {
					if err := networkpolicies.Create(ctx, kube, owner, nsSpec); err != nil {
						return fmt.Errorf("networkPolicies[%d].create namespace=%s: %w", i, targetNs, err)
					}
				}
			}
			continue
		}

		if update {
			if err := networkpolicies.Update(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("networkPolicies[%d].update: %w", i, err)
			}
		} else {
			if err := networkpolicies.Create(ctx, kube, owner, spec); err != nil {
				return fmt.Errorf("networkPolicies[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := networkpolicies.Update(ctx, kube, owner, spec); err != nil {
					return fmt.Errorf("networkPolicies[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
