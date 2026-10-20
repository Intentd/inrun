// pkg/runners/networkpolicies.go
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
	"github.com/intentd/kubewright/core/networkpolicies"
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
					res, err := networkpolicies.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("networkPolicies[%d]: conditional cleanup: %w", i, err)
					}
					logResult("networkpolicy", name, ns, owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "NetworkPolicy").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveNetworkPolicy(resolver, src)
		if err != nil {
			return fmt.Errorf("networkPolicies[%d]: %w", i, err)
		}

		spec := resources.NetworkPolicySpec(resolved, owner, resolver.Presets())

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
					res, err := networkpolicies.Apply(ctx, kube.Clientset(), owner, nsSpec, kubeclient.ApplyOptions(kube, src.ForceConflict))
					if err != nil {
						return fmt.Errorf("networkPolicies[%d].update namespace=%s: %w", i, targetNs, err)
					}
					logResult("networkpolicy", nsSpec.Name, nsSpec.Namespace, owner, res)
				} else {
					res, err := networkpolicies.Create(ctx, kube.Clientset(), owner, nsSpec)
					if err != nil {
						return fmt.Errorf("networkPolicies[%d].create namespace=%s: %w", i, targetNs, err)
					}
					logResult("networkpolicy", nsSpec.Name, nsSpec.Namespace, owner, res)
				}
			}
			continue
		}

		if update {
			res, err := networkpolicies.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("networkPolicies[%d].update: %w", i, err)
			}
			logResult("networkpolicy", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := networkpolicies.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("networkPolicies[%d].create: %w", i, err)
			}
			logResult("networkpolicy", spec.Name, spec.Namespace, owner, res)
			if src.Reconcile {
				res, err := networkpolicies.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("networkPolicies[%d].reconcile: %w", i, err)
				}
				logResult("networkpolicy", spec.Name, spec.Namespace, owner, res)
			}
		}
	}
	return nil
}
