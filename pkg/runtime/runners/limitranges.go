// pkg/runners/limitranges.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/limitranges"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunLimitRanges resolves and applies LimitRange template declarations.
//
// limitRanges support fromLimitRange to copy an existing LimitRange's limits,
// and toNamespaces to distribute copies across multiple namespaces.
//
// reconcile: true — re-reads the source LimitRange on every reconcile and syncs changes.
func RunLimitRanges(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.LimitRangeTemplateSource,
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
					if err := limitranges.DeleteIfOwned(ctx, kube.Clientset(), owner, name, ns); err != nil {
						return fmt.Errorf("limitRanges[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "LimitRange").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveLimitRange(resolver, src)
		if err != nil {
			return fmt.Errorf("limitRanges[%d]: %w", i, err)
		}

		spec := resources.LimitRangeSpec(resolved, resolver.OwnerName(), resolver.Presets())

		if len(resolved.ToNamespaces) > 0 {
			namespaces, err := resolver.ResolveStringSlice(resolved.ToNamespaces)
			if err != nil {
				return fmt.Errorf("limitRanges[%d].toNamespaces: %w", i, err)
			}

			shouldSync := update || src.Reconcile
			for _, targetNs := range namespaces {
				if guard != nil && !guard(ctx, owner, targetNs) {
					continue
				}
				nsSpec := spec
				nsSpec.Namespace = targetNs
				if shouldSync {
					if err := limitranges.Apply(ctx, kube.Clientset(), owner, nsSpec, kubeclient.ApplyOptions(kube, nsSpec.ForceConflict)); err != nil {
						return fmt.Errorf("limitRanges[%d].update namespace=%s: %w", i, targetNs, err)
					}
				} else {
					if err := limitranges.Create(ctx, kube.Clientset(), owner, nsSpec); err != nil {
						return fmt.Errorf("limitRanges[%d].create namespace=%s: %w", i, targetNs, err)
					}
				}
			}
			continue
		}

		if update {
			if err := limitranges.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
				return fmt.Errorf("limitRanges[%d].update: %w", i, err)
			}
		} else {
			if err := limitranges.Create(ctx, kube.Clientset(), owner, spec); err != nil {
				return fmt.Errorf("limitRanges[%d].create: %w", i, err)
			}
			if src.Reconcile {
				if err := limitranges.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
					return fmt.Errorf("limitRanges[%d].reconcile: %w", i, err)
				}
			}
		}
	}
	return nil
}
