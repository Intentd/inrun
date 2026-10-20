// pkg/runners/replicasets.go
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
	"github.com/intentd/kubewright/core/replicasets"
)

// RunReplicaSets resolves and applies ReplicaSet template declarations.
//
// update=false  onCreate path  — idempotent Create
// update=true   onReconcile path — Apply for drift correction
//
// reconcile: true on an onCreate entry means also call Apply on that
// same reconcile loop — the shorthand for "create it and keep it in sync"
// without a separate onReconcile declaration.
func RunReplicaSets(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ReplicaSetTemplateSource,
	update bool,
	guard func(ctx context.Context, obj domain.Object, ns string) bool,
) error {

	// Track active names for conditional cleanup
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
		name, _ := resolver.Resolve(src.Name)
		ns, _ := resolver.Resolve(src.Namespace)
		if ns == "" {
			ns = owner.GetNamespace()
		}

		// ── Namespace guard ───────────────────────────────────────────────────
		if guard != nil && !guard(ctx, owner, ns) {
			continue
		}

		logger.FromContext(ctx).Debug().
			Str("resource", "ReplicaSet").
			Str("name", name).
			Str("namespace", ns).
			Bool("namespace_restricted", guard != nil).
			Int("index", i).
			Bool("condition_passed", conditionPassed).
			Msg("replicaset: condition evaluation")

		// ── Conditional cleanup ───────────────────────────────────────────────
		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					res, err := replicasets.DeleteIfOwned(ctx, kube.Clientset(), name, ns, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("replicasets[%d]: conditional cleanup: %w", i, err)
					}
					logResult("replicaset", name, ns, owner, res)
				}
			}
			continue
		}

		// 2. Resolve template expressions
		resolved, err := resources.ResolveReplicaSet(resolver, src)
		if err != nil {
			return fmt.Errorf("replicasets[%d]: %w", i, err)
		}

		spec := resources.ReplicaSetSpec(resolved, owner, resolver.Presets())

		if update {
			res, err := replicasets.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
			if err != nil {
				return fmt.Errorf("replicasets[%d].update: %w", i, err)
			}
			logResult("replicaset", spec.Name, spec.Namespace, owner, res)
		} else {
			res, err := replicasets.Create(ctx, kube.Clientset(), owner, spec)
			if err != nil {
				return fmt.Errorf("replicasets[%d].create: %w", i, err)
			}
			logResult("replicaset", spec.Name, spec.Namespace, owner, res)

			// reconcile: true
			if src.Reconcile {
				res, err := replicasets.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, src.ForceConflict))
				if err != nil {
					return fmt.Errorf("replicasets[%d].reconcile: %w", i, err)
				}
				logResult("replicaset", spec.Name, spec.Namespace, owner, res)
			}
		}

		// Workload autoscale — evaluated on every reconcile after create/update.
		if src.Autoscale != nil {
			if err := EvaluateWorkloadAutoscaleReplicaSet(ctx, kube, resolver,
				owner.GetName(), ns, name, src.Autoscale); err != nil {
				return fmt.Errorf("replicasets[%d].autoscale: %w", i, err)
			}
		}
	}

	return nil
}
