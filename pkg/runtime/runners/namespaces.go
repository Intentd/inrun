// pkg/runners/namespaces.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/namespaces"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// RunNamespaces resolves and applies Namespace template declarations.
//
// Namespaces are create-only — there is nothing meaningful to update
// on a Namespace after creation (no spec fields that drift). They are
// therefore always idempotent creates regardless of whether this is called
// from onCreate or onReconcile.
//
// Owner references ensure cleanup when the CR is deleted.
func RunNamespaces(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.NamespaceTemplateSource,
	update bool,
) error {
	activeNames := make(map[string]bool, len(srcs))
	for _, s := range srcs {
		if !types.EvaluateConditions(resolver.Data(), s.Conditions, s.Or, resolver.TemplateEvaluator()) {
			continue
		}
		n, _ := resolver.Resolve(s.Name)
		activeNames["inrun.dev"+"/"+n] = true
	}

	for i, src := range srcs {
		// 1. Evaluate conditions BEFORE resolving templates
		conditionPassed := types.EvaluateConditions(resolver.Data(), src.Conditions, src.Or, resolver.TemplateEvaluator())

		// Early name resolution — needed for DeleteIfOwned cleanup.
		name, _ := resolver.Resolve(src.Name)

		if !conditionPassed {
			if update || src.Reconcile {
				if !activeNames["inrun.dev"+"/"+name] {
					res, err := namespaces.DeleteIfOwned(ctx, kube.Clientset(), name, labels.OwnedBy(owner))
					if err != nil {
						return fmt.Errorf("namespace[%d]: conditional cleanup: %w", i, err)
					}
					logResult("namespace", name, "", owner, res)
				}
			}
			logger.FromContext(ctx).Debug().
				Str("resource", "Namespace").
				Int("index", i).
				Msg("conditions not met — skipping resource")

			continue
		}

		// 2. Resolve template expressions
		resolved, err := resources.ResolveNamespace(resolver, src)
		if err != nil {
			return fmt.Errorf("namespace[%d]: %w", i, err)
		}

		// 3. Build registry spec and apply
		spec := resources.NamespaceSpec(resolved, owner)

		// Always create — Namespaces have no meaningful drift to correct
		res, err := namespaces.Create(ctx, kube.Clientset(), owner, spec)
		if err != nil {
			return fmt.Errorf("namespace[%d].create: %w", i, err)
		}
		logResult("namespace", spec.Name, "", owner, res)
	}
	return nil
}
