package reconcile

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/runtime/runners"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Lifecycle steps the runtime runs for every reconciler kind. A reconciler
// decides what its CR needs; when a CR's children go is Inrun's job.

// deleteChildren removes the children owner references cannot reach once a
// finalizing reconcile is done: every child a Result applied for the CR (any
// namespace, cluster-scoped included, except prune.keep kinds) and the
// cluster-scoped children its templates declare.
func deleteChildren(ctx context.Context, kube kubeclient.Interface, crd types.CRDEntry, prepared *domain.PreparedRequest) ([]apply.Pruned, error) {
	var keep []string
	if p := crd.EffectivePrune(prepared.Target); p != nil {
		keep = p.Keep
	}
	deleted, err := apply.DeleteChildren(ctx, kube.DynamicClient(), prepared.Object, crd.AllManagedResources(), keep)
	if err != nil {
		return deleted, err
	}
	if resolver, ok := prepared.Context.(*template.Resolver); ok {
		if err := runners.DeleteOwnedClusterScopedResources(ctx, kube, resolver, prepared.Object, prepare.BoxFrom(prepared)); err != nil {
			return deleted, fmt.Errorf("cluster-scoped children: %w", err)
		}
	}
	return deleted, nil
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

	if prev != "" {
		key := labels.EffectiveOwnerKey(obj.GetName(), map[string]string{labels.AnnotationServeAlias: prev})
		if err := runners.SweepOwnedNamespacedResources(ctx, kube, key, obj.GetNamespace()); err != nil {
			return err
		}
		if err := runners.SweepOwnedClusterScopedResources(ctx, kube, key); err != nil {
			return err
		}
		if err := runners.SweepOwnedManagedResources(ctx, kube.DynamicClient(), crd.AllManagedResources(), key); err != nil {
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
