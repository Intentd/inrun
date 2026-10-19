// pkg/runners/configmaps.go
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
	"github.com/intentd/kuberesources/core/configmaps"
)

// RunConfigMaps resolves and applies ConfigMap template declarations.
//
// ConfigMaps support the same fromConfigMap and toNamespaces patterns as Secrets:
//
// fromConfigMap — copies data from an existing ConfigMap, with declared
// data entries overriding matching keys from the source. This is the
// "base config + environment override" pattern.
//
// toNamespaces — creates one copy in each listed namespace.
//
// reconcile: true — re-reads the source ConfigMap on every reconcile
// and syncs any changes. When logLevel changes in the CR spec, the
// ConfigMap in every namespace updates automatically.
func RunConfigMaps(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ConfigMapTemplateSource,
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

		// Early name/ns resolution — needed for guard check and DeleteIfOwned cleanup.
		// resources.ResolveConfigMap resolves these again internally — intentional, cheap.
		name, _ := resolver.Resolve(src.Name)
		ns, _ := resolver.Resolve(src.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}

		// ── Namespace guard ───────────────────────────────────────────────────
		if guard != nil && !guard(ctx, owner, ns) {
			continue // skipped — CheckNamespace already logged the reason
		}

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					res, err := configmaps.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("configMaps[%d]: conditional cleanup: %w", i, err)
					}
					logResult("configmap", name, ns, owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "ConfigMap").
				Int("index", i).
				Msg("conditions not met — skipping resource")

			continue
		}

		// 2. Resolve template expressions
		resolved, err := resources.ResolveConfigMap(resolver, src)
		if err != nil {
			return fmt.Errorf("configmaps[%d]: %w", i, err)
		}

		// 3. Build registry spec and apply
		spec := resources.ConfigMapSpec(resolved, owner)

		// toNamespaces — distribute to multiple namespaces, guarded per target
		if len(resolved.ToNamespaces) > 0 {
			namespaces, err := resolver.ResolveStringSlice(resolved.ToNamespaces)
			if err != nil {
				return fmt.Errorf("configmaps[%d].toNamespaces: %w", i, err)
			}

			shouldSync := update || src.Reconcile
			for _, targetNs := range namespaces {
				// Guard per target namespace — skip restricted, continue to allowed
				if guard != nil && !guard(ctx, owner, targetNs) {
					continue
				}
				nsSpec := spec
				nsSpec.Namespace = targetNs
				if shouldSync {
					res, err := configmaps.Apply(ctx, kube.Clientset(), owner, nsSpec, kubeclient.ApplyOptions(kube, src.ForceConflict))
					if err != nil {
						return fmt.Errorf("configmaps[%d].update namespace=%s: %w", i, targetNs, err)
					}
					logResult("configmap", nsSpec.Name, nsSpec.Namespace, owner, res)
				} else {
					res, err := configmaps.Create(ctx, kube.Clientset(), owner, nsSpec)
					if err != nil {
						return fmt.Errorf("configmaps[%d].create namespace=%s: %w", i, targetNs, err)
					}
					logResult("configmap", nsSpec.Name, nsSpec.Namespace, owner, res)
				}
			}
			continue
		}

		// Single namespace
		if update {
			res, err := configmaps.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("configmaps[%d].update: %w", i, err)
			}
			logResult("configmap", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := configmaps.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("configmaps[%d].create: %w", i, err)
			}
			logResult("configmap", spec.Name, spec.Namespace, owner, res)
			if src.Reconcile {
				res, err := configmaps.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("configmaps[%d].reconcile: %w", i, err)
				}
				logResult("configmap", spec.Name, spec.Namespace, owner, res)
			}
		}
	}
	return nil
}
