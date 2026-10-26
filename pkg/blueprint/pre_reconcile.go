package blueprint

import (
	"context"

	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// compile time check
var _ domain.Blueprint = (*Blueprint)(nil)

// EvaluatePreReconcile evaluates preReconcile.reconcileGate conditions for the named CRD.
// Returns (true, "") when conditions pass and the reconciler should run.
// Returns (false, reason) when gated — reconciler must not be called.
//
// preReconcile.external runs first (shared enrichment), then reconcileGate.external,
// then conditions are evaluated against the accumulated resolver.
func (c *Blueprint) EvaluatePreReconcile(ctx context.Context, gvk string, obj *unstructured.Unstructured, cs kubernetes.Interface, opts domain.EvaluateOptions) (allowed bool, reason string) {
	box := c.effectiveBox(obj, gvk)
	if box.Empty() {
		return true, ""
	}

	pr := box.PreReconcile
	if pr.Empty() || !pr.HasReconcileGate() {
		return true, ""
	}

	resolver := c.effectiveResolver(ctx, obj, pr, opts)
	if resolver.Empty() {
		return true, ""
	}

	var err error
	if pr.HasPreReconcileExternal() {
		if resolver, err = external.Run(ctx, gvk, resolver, pr.External, cs); err != nil {
			return true, "" // shared pre-reconcile external: always fail-open
		}
	}
	g := pr.ReconcileGate
	if pr.HasReconcileGateExternal() {
		if resolver, err = external.Run(ctx, gvk, resolver, g.External, cs); err != nil {
			if g.FailPolicy == types.FailPolicyClosed {
				return false, "reconcileGate: external evaluation failed (failPolicy: closed)"
			}
			return true, ""
		}
	}

	// Evaluate reconcileGate.sentinels (shorthand) - first match wins
	if g.HasSentinels() {
		return g.SentinelsAllowed(opts.Sentinels), ""
	}

	return resolver.EvaluateConditions(pr.WhenConditions(), pr.OrConditions())
}

// EvaluateEnqueueFilter evaluates preReconcile.enqueueGate conditions for the named CRD.
// Returns true when the object should be enqueued, false when it should be dropped.
// Accepts domain.Object so it works for both dynamic and typed CRDs.
//
// preReconcile.external runs first (shared enrichment), then enqueueGate.external,
// then conditions are evaluated against the accumulated resolver.
func (c *Blueprint) EvaluateEnqueueFilter(ctx context.Context, gvk string, obj domain.Object, cs kubernetes.Interface, opts domain.EvaluateOptions) bool {
	box := c.effectiveBox(obj, gvk)
	if box.Empty() {
		return true
	}

	pr := box.PreReconcile
	if pr.Empty() || !pr.HasEnqueueGate() {
		return true
	}

	resolver := c.effectiveResolver(ctx, obj, pr, opts)
	if resolver.Empty() {
		return true
	}

	var err error
	if pr.HasPreReconcileExternal() {
		if resolver, err = external.Run(ctx, gvk, resolver, pr.External, cs); err != nil {
			return true // shared pre-reconcile external: always fail-open
		}
	}

	return c.evaluateGate(ctx, gvk, pr.EnqueueGate, resolver, cs, opts.Sentinels)
}

// EvaluateWatchEnqueueFilter evaluates a watch entry's enqueueGate.
func (c *Blueprint) EvaluateWatchEnqueueFilter(ctx context.Context, primaryGVK, secondaryGVK string, obj domain.Object, cs kubernetes.Interface, opts domain.EvaluateOptions) bool {
	box := c.effectiveBox(obj, primaryGVK)
	if box == nil {
		return true
	}

	resolver := c.effectiveResolver(ctx, obj, nil, opts)
	if resolver.Empty() {
		return true
	}

	entry := c.LookupByGVKString(primaryGVK).Entry()
	if entry == nil {
		return true
	}

	watchEntry := box.GetWatchEntry(secondaryGVK)
	if watchEntry == nil || watchEntry.EnqueueGate == nil {
		return true
	}

	return c.evaluateGate(ctx, secondaryGVK, watchEntry.EnqueueGate, resolver, cs, opts.Sentinels)
}

// EvaluateEventEnqueueFilter evaluates an event entry's enqueueGate.
func (c *Blueprint) EvaluateEventEnqueueFilter(ctx context.Context, primaryGVK, eventName string, obj domain.Object, cs kubernetes.Interface, opts domain.EvaluateOptions) bool {
	box := c.effectiveBox(obj, primaryGVK)
	if box == nil {
		return true
	}

	resolver := c.effectiveResolver(ctx, obj, nil, opts)
	if resolver.Empty() {
		return true
	}

	entry := c.LookupByGVKString(primaryGVK).Entry()
	if entry == nil {
		return true
	}

	eventEntry := box.GetEventEntry(eventName)
	if eventEntry == nil || eventEntry.EnqueueGate == nil {
		return true
	}

	return c.evaluateGate(ctx, eventName, eventEntry.EnqueueGate, resolver, cs, opts.Sentinels)
}

// EvaluateQueueBehaviourConditions completes the queue behaviour evaluation started by the
// workqueue but delegated to the informer. Evaluates queue.behaviour conditions for the named CRD.
// Returns true when the object should be enqueued, false when it should be dropped.
// Accepts domain.Object so it works for both dynamic and typed CRDs.
//
// Here because it influences 'pre-reconcile' decisions.
func (c *Blueprint) EvaluateQueueBehaviourConditions(ctx context.Context, gvk string, obj domain.Object, opts domain.EvaluateOptions) bool {
	box := c.effectiveBox(obj, gvk)
	if box.Empty() {
		return true
	}

	rc := box.Reconcile
	q := rc.Queue
	if rc.Empty() || q.Empty() || !q.HasBehaviourCondition() {
		return true
	}

	resolver := c.effectiveResolver(ctx, obj, box.PreReconcile, opts)
	if resolver == nil {
		return true
	}

	if q.HasOnLimitConditions() {
		return types.EvaluateConditions(resolver.Data(), q.OnLimitWhen(), q.OnLimitOr(), resolver.TemplateEvaluator())
	}
	if q.HasOnThresholdConditions() {
		return types.EvaluateConditions(resolver.Data(), q.OnThresholdWhen(), q.OnThresholdOr(), resolver.TemplateEvaluator())
	}

	return true
}

// effectiveResolver computes the common resolver used by all evaluators
func (c *Blueprint) effectiveResolver(ctx context.Context, obj domain.Object, pr *types.PreReconcileConfig, opts domain.EvaluateOptions) *template.Resolver {
	resolver, err := template.NewResolver(ctx, obj)
	if err != nil {
		return nil
	}
	if !c.Presets.Empty() {
		resolver = resolver.WithPresets(c.UserPresets())
	}
	if !c.Functions.Empty() {
		resolver = resolver.WithUserFunctions(c.UserFunctions())
	}
	// .request context
	if req := intent.FromObject(resolver.Data()); req != nil {
		resolver = resolver.WithIntent(req)
	}
	// .metrics context
	if metrics := labels.ResolveResourceMetricFromObject(resolver.Data()); metrics != nil {
		resolver = resolver.WithMetrics(metrics)
	}
	// .health context
	if health := labels.ResolveResourceHealthFromObject(resolver.Data()); health != nil {
		resolver = resolver.WithHealth(health)
	}
	// .events context
	if len(opts.Events) > 0 {
		resolver = resolver.WithEvents(opts.Events)
	}
	if len(opts.Sentinels) > 0 {
		resolver = resolver.WithSentinels(pr.DeclaredSentinels(), opts.Sentinels)
	}
	return resolver
}

// effectiveBox computes the common operatorBox used by all evaluators
func (c *Blueprint) effectiveBox(obj domain.Object, gvk string) *types.OperatorBoxConfig {
	if obj == nil {
		return nil
	}
	entry := c.LookupByGVKString(gvk).Entry()
	if entry == nil {
		return nil
	}
	target := intent.Target(obj.GetAnnotations())
	return entry.EffectiveOperatorBox(target)
}

// evaluateGate evaluates a gate against the supplied resolver.
//
// Gate external runs first, then sentinel shorthand, then conditions.
func (c *Blueprint) evaluateGate(
	ctx context.Context,
	gvk string,
	g *types.GateConditions,
	resolver *template.Resolver,
	cs kubernetes.Interface,
	sentinels map[string]string,
) bool {
	if resolver == nil {
		return true
	}

	var err error
	if g.HasExternal() {
		if resolver, err = external.Run(ctx, gvk, resolver, g.External, cs); err != nil {
			if g.FailPolicy == types.FailPolicyClosed {
				return false
			}
			return true
		}
	}

	// Sentinel shorthand takes precedence over conditions.
	if g.HasSentinels() {
		return g.SentinelsAllowed(sentinels)
	}

	return types.EvaluateConditions(
		resolver.Data(),
		g.WhenConditions(),
		g.OrConditions(),
		resolver.TemplateEvaluator(),
	)
}

// IsEventAware reports whether the named CRD has opted into event-aware
// reconcileGate evaluation.
//
// When true, events entering the CRD's workqueue must retain their individual
// event identity rather than being coalesced with other events for the same
// object. This applies to the entire reconcileGate evaluation, not only
// sentinel conditions.
func (c *Blueprint) IsEventAware(obj domain.Object, name string) bool {
	if c == nil {
		return false
	}

	box := c.effectiveBox(obj, name)
	if box == nil {
		return false
	}

	pr := box.PreReconcile
	if pr == nil {
		return false
	}

	if pr.HasReconcileGate() {
		return pr.ReconcileGate.IsEventAware()
	}

	return false
}

// GetPreReconcileSentinels returns the sentinel names declared by
// preReconcile.sentinels for the named CRD.
//
// The informer uses this declaration to compute event-time sentinel values
// from old and new objects. Sentinel declaration is owned by the Blueprint;
// the informer does not maintain a separate sentinel configuration registry.
// Returns nil when the CRD is unknown or declares no sentinels.
func (c *Blueprint) GetPreReconcileSentinels(obj domain.Object, gvkString string) []string {
	if c == nil {
		return nil
	}
	box := c.effectiveBox(obj, gvkString)
	if box == nil {
		return nil
	}

	pr := box.PreReconcile
	if pr == nil {
		return nil
	}

	return pr.DeclaredSentinels()
}
