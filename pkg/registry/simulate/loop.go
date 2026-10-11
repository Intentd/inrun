package simulate

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"k8s.io/client-go/tools/cache"
)

// loopKube is the kubeclient capability the reconcile loop requires beyond
// the standard interface: cycle tracking, op collection, and the
// deployment-ready helper needed when no controller-manager runs.
type loopKube interface {
	kubeclient.Interface
	AdvanceCycle()
	OpsForCycle(cycle int) []Op
	Ops() []Op
	MarkDeploymentReady(namespace, name string)
}

// runLoop runs one reconcile pass per cycle for up to maxCycles — the same
// reconcile.Run the runtime's workers call — and returns the aggregate result.
// What the runtime gets from its informer and a controller-manager happens
// here instead: the enqueue gate before each pass, and Deployments marked
// ready after it. Notes must be appended by the caller.
func runLoop(ctx context.Context, deps reconcile.Deps, in reconcile.Input, kube loopKube, maxCycles int) *Result {
	result := &Result{}
	var prevCycleOps []Op

	for cycle := 1; cycle <= maxCycles; cycle++ {
		kube.AdvanceCycle()
		cycleResult := CycleResult{Cycle: cycle}

		if enqueued(ctx, deps, in) {
			cycleResult.Error = reconcile.Run(ctx, deps, in).Err
		}

		cycleResult.Ops = kube.OpsForCycle(cycle)
		result.Cycles = append(result.Cycles, cycleResult)

		// Mark Deployments created this cycle as ready so the reconciler
		// can progress through state transitions on the next cycle
		// (no controller-manager runs in fake or envtest mode).
		for _, op := range cycleResult.Ops {
			if op.Verb == "create" && op.Resource == "deployments" {
				kube.MarkDeploymentReady(op.Namespace, op.Name)
			}
		}

		if !result.Steady && cycle > 1 && opsMatch(cycleResult.Ops, prevCycleOps) {
			result.Steady = true
			result.SteadyAt = cycle
		}
		prevCycleOps = cycleResult.Ops
	}

	result.AllOps = kube.Ops()
	return result
}

// enqueued admits the key the way the runtime's informer does before it is
// queued (informer.AdmitPrimary): the CRD's namespace rules and
// preReconcile.enqueueGate. There is no live queue, so its behaviour
// conditions do not apply.
func enqueued(ctx context.Context, deps reconcile.Deps, in reconcile.Input) bool {
	if in.Entry.Informer == nil {
		return true
	}
	raw, exists, err := in.Entry.Informer.GetIndexer().GetByKey(in.Key)
	if err != nil || !exists {
		return true
	}
	crd := in.Entry.CRD
	return informer.AdmitPrimary(ctx, informer.Admission{
		Catalog:   deps.Cat,
		Clientset: deps.Kube.Clientset(),
		Namespace: &informer.NamespaceFilter{
			AllowedNamespaces:    []string(crd.AllAllowedNamespaces()),
			RestrictedNamespaces: []string(crd.AllRestrictedNamespaces()),
		},
	}, in.GVK, raw, nil)
}

// compile-time check that *FakeKubeclient satisfies loopKube.
var _ loopKube = (*FakeKubeclient)(nil)

// compile-time check that cache.SharedIndexInformer is accessible from cache.
var _ cache.SharedIndexInformer = (*fakeInformer)(nil)
