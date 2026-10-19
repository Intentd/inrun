// pkg/runners/statefulsets.go
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
	"github.com/intentd/kuberesources/core/statefulsets"
)

// RunStatefulSets resolves and applies StatefulSet template declarations.
func RunStatefulSets(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.StatefulSetTemplateSource,
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
					res, err := statefulsets.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("statefulsets[%d]: conditional cleanup: %w", i, err)
					}
					logResult("statefulset", name, ns, owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "StatefulSet").
				Int("index", i).
				Msg("conditions not met — skipping resource")
			continue
		}

		resolved, err := resources.ResolveStatefulSet(resolver, src)
		if err != nil {
			return fmt.Errorf("statefulsets[%d]: %w", i, err)
		}

		spec := resources.StatefulSetSpec(resolved, owner, resolver.Presets())

		if update {
			res, err := statefulsets.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("statefulsets[%d].update: %w", i, err)
			}
			logResult("statefulset", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := statefulsets.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("statefulsets[%d].create: %w", i, err)
			}
			logResult("statefulset", spec.Name, spec.Namespace, owner, res)
			if src.Reconcile {
				res, err := statefulsets.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("statefulsets[%d].reconcile: %w", i, err)
				}
				logResult("statefulset", spec.Name, spec.Namespace, owner, res)
			}
		}

		// Workload autoscale — evaluated on every reconcile after create/update.
		if src.Autoscale != nil {
			if err := EvaluateWorkloadAutoscaleStatefulSet(ctx, kube, resolver,
				owner.GetName(), ns, name, src.Autoscale); err != nil {
				return fmt.Errorf("statefulsets[%d].autoscale: %w", i, err)
			}
		}
	}
	return nil
}
