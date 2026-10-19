// pkg/runners/pods.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/pods"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunPods resolves and applies Pod template declarations.
//
// update=false  onCreate path  — idempotent Create
// update=true   onReconcile path — Apply for drift correction (delete + recreate on image drift)
//
// reconcile: true on an onCreate entry means also call Apply on that
// same reconcile loop — the shorthand for "create it and keep it in sync"
// without a separate onReconcile declaration.
func RunPods(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PodTemplateSource,
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

		logger.FromContext(ctx).Debug().
			Str("resource", "Pod").
			Str("name", name).
			Str("namespace", ns).
			Int("index", i).
			Bool("condition_passed", conditionPassed).
			Msg("pod: condition evaluation")

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames[ns+"/"+name] {
					if err := pods.DeleteIfOwned(ctx, kube.Clientset(), owner, name, ns); err != nil {
						return fmt.Errorf("pods[%d]: conditional cleanup: %w", i, err)
					}
				}
			}
			continue
		}

		resolved, err := resources.ResolvePod(resolver, src)
		if err != nil {
			return fmt.Errorf("pods[%d]: %w", i, err)
		}

		spec := resources.PodSpec(resolved, resolver.OwnerName(), resolver.Presets())

		if update {
			if err := pods.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
				return fmt.Errorf("pods[%d].update: %w", i, err)
			}
		} else {
			if err := pods.Create(ctx, kube.Clientset(), owner, spec); err != nil {
				return fmt.Errorf("pods[%d].create: %w", i, err)
			}

			if src.Reconcile {
				if err := pods.Apply(ctx, kube.Clientset(), owner, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
					return fmt.Errorf("pods[%d].reconcile: %w", i, err)
				}
			}
		}
	}

	return nil
}
