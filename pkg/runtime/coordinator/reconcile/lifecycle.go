package reconcile

import (
	"context"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply/templates"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Lifecycle steps the runtime runs for every reconciler kind. A reconciler
// decides what its CR needs; when a CR's children go is Inrun's job.

// pruneChildren removes a CR's children once a finalizing reconcile is
// done: every object carrying its UID label (labels.OwnerUID), whoever wrote
// it, in any namespace and cluster-scoped ones included, except prune.keep
// kinds. Owner references reach only same-namespace children; this reaches
// the rest.
func pruneChildren(ctx context.Context, kube kubeclient.Interface, crd types.CRDEntry, prepared *domain.PreparedRequest) ([]apply.Pruned, error) {
	var keep []string
	if p := crd.EffectivePrune(prepared.Target); p != nil {
		keep = p.Keep
	}
	return apply.PruneChildren(ctx, kube.DynamicClient(), prepared.Object, childKinds(kube, crd), keep)
}

// childKinds are the kinds a CR's children can be: the managed kinds a
// Result may return, and the kinds the CRD's templates write.
func childKinds(kube kubeclient.Interface, crd types.CRDEntry) []schema.GroupVersionResource {
	gvrs := apply.ManagedGVRs(crd.AllManagedResources())
	seen := make(map[schema.GroupVersionResource]bool, len(gvrs))
	for _, g := range gvrs {
		seen[g] = true
	}
	for _, g := range templates.Kinds(crd, kube.RESTMapper()) {
		if !seen[g] {
			seen[g] = true
			gvrs = append(gvrs, g)
		}
	}
	return gvrs
}

// sweepPreviousSurface deletes what a CR's previous serve target created once
// it has moved to another target, unless the target keeps its previous
// surface, then records the current target in the last-surface annotation.
// It runs after a successful reconcile, so the new target's children exist
// before the old ones go.
func sweepPreviousSurface(ctx context.Context, kube kubeclient.Interface, crd types.CRDEntry, obj domain.Object) error {
	target := intent.Target(obj.GetAnnotations())
	if target == "" || crd.KeepPreviousSurface(target) {
		return nil
	}
	prev := obj.GetAnnotations()[labels.AnnotationLastSurface]
	if prev == target {
		return nil
	}

	if prev != "" && obj.GetUID() != "" {
		selector := labels.OwnerUID + "=" + string(obj.GetUID()) + "," + labels.InrunServeTarget + "=" + prev
		if _, err := apply.PruneOwned(ctx, kube.DynamicClient(), selector, childKinds(kube, crd), nil); err != nil {
			return err
		}
	}

	if err := kube.PatchAnnotations(ctx, obj, map[string]string{labels.AnnotationLastSurface: target}, metav1.PatchOptions{}); err != nil {
		logger.FromContext(ctx).Warn().Err(err).
			Str("name", obj.GetName()).
			Msg("surface sweep: failed to record last-surface")
	}
	return nil
}

// finalizeDeadline reports whether obj has been finalizing for at least
// runtime.finalizers.timeout, counted from its deletionTimestamp, and how long
// is left otherwise. With no timeout set it never expires and left is 0.
func finalizeDeadline(obj domain.Object, cfg *types.FinalizerConfig, now time.Time) (expired bool, left time.Duration) {
	ts, timeout := obj.GetDeletionTimestamp(), cfg.TimeoutDuration()
	if timeout <= 0 || ts == nil {
		return false, 0
	}
	left = ts.Add(timeout).Sub(now)
	if left <= 0 {
		return true, 0
	}
	return false, left
}
