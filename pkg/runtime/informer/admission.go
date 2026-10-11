package informer

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"k8s.io/client-go/kubernetes"
)

// Admission is what an event is checked against before it is queued: the
// CRD's namespace rules, the queue's behaviour conditions and, for primary
// events, preReconcile.enqueueGate. inrun simulate builds one per CRD so its
// passes are admitted exactly as the runtime's are.
type Admission struct {
	Catalog   domain.Catalog
	Clientset kubernetes.Interface
	// Namespace is the CRD's namespace rules; nil or inactive allows all.
	Namespace *NamespaceFilter
	// Queue is the CRD's workqueue; nil skips its behaviour conditions.
	Queue *queue.Workqueue
}

// AdmitPrimary reports whether a primary event for obj is queued.
func AdmitPrimary(ctx context.Context, a Admission, gvkStr string, obj interface{}, sentinels map[string]string) bool {
	domObj, verdict, decided := admitCommon(ctx, a, gvkStr, obj, sentinels)
	if decided {
		return verdict
	}
	if !a.Catalog.EvaluateEnqueueFilter(ctx, gvkStr, domObj, a.Clientset, domain.EvaluateOptions{Sentinels: sentinels}) {
		logger.Debug().
			Str("gvk", gvkStr).
			Str("name", domObj.GetName()).
			Msg("informer: event dropped — enqueue filter")
		return false
	}
	return true
}

// admitCommon applies the checks every event goes through. When decided is
// true, verdict is final; otherwise the source-specific checks follow.
func admitCommon(ctx context.Context, a Admission, gvkStr string, obj interface{}, sentinels map[string]string) (domObj domain.Object, verdict, decided bool) {
	// Namespace restriction.
	namespace := domain.ExtractNamespace(obj)
	if a.Namespace.IsActive() && !a.Namespace.Allows(namespace) {
		logger.Debug().
			Str("gvk", gvkStr).
			Str("namespace", namespace).
			Msg("informer: event dropped — namespace not allowed")
		return nil, false, true
	}

	domObj, ok := domain.ToDomainObject(obj)
	if !ok || a.Catalog == nil {
		return domObj, true, true
	}

	// A CR being deleted always gets through: its finalizing reconcile is what
	// releases Inrun's finalizer, so no condition may hold it back.
	if domObj.GetDeletionTimestamp() != nil {
		return domObj, true, true
	}

	// Queue behaviour conditions.
	if a.Queue != nil && a.Queue.NeedsBehaviourEval() {
		if !a.Catalog.EvaluateQueueBehaviourConditions(ctx, gvkStr, domObj, domain.EvaluateOptions{Sentinels: sentinels}) {
			return domObj, false, true
		}
	}
	return domObj, false, false
}

// admission is this factory's Admission for gvkStr and its queue.
func (f *Factory) admission(gvkStr string, wq *queue.Workqueue) Admission {
	f.mu.RLock()
	filter := f.namespaceFilters[gvkStr]
	f.mu.RUnlock()
	return Admission{Catalog: f.catalog, Clientset: f.cs, Namespace: filter, Queue: wq}
}
