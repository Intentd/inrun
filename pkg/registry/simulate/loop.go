package simulate

import (
	"context"
	"fmt"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// loopKube is the kubeclient capability the reconcile loop requires beyond
// the standard interface: cycle tracking, op collection, and the
// deployment-ready helper needed when no controller-manager runs.
type loopKube interface {
	kubeclient.Interface
	AdvanceCycle()
	OpsForCycle(cycle int) Ops
	Ops() Ops
	MarkDeploymentReady(namespace, name string)
	// Object reads one object as it stands now, without recording an op.
	Object(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error)
	// CatchUp brings the informer's copy of one object up to what the
	// cluster holds, as a real informer catches up between reconciles, so
	// the next cycle prepares from what this one wrote.
	CatchUp(ctx context.Context, inf cache.SharedIndexInformer, gvr schema.GroupVersionResource, namespace, name string) error
}

// runLoop runs one reconcile pass per cycle for up to maxCycles — the same
// reconcile.Run the runtime's workers call — and returns the aggregate result.
// What the runtime gets from its informer and a controller-manager happens
// here instead: the enqueue gate before each pass, and Deployments marked
// ready after it. Notes must be appended by the caller.
func runLoop(ctx context.Context, deps reconcile.Deps, in reconcile.Input, kube loopKube, maxCycles int) *Result {
	result := &Result{}
	var prevCycleOps Ops

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

		// The next cycle prepares from the informer's copy of the CR: bring
		// it up to what this cycle wrote, in both backends, so a cycle never
		// reads a stale CR and writes again what it already wrote.
		if ns, name, err := cache.SplitMetaNamespaceKey(in.Key); err == nil {
			if err := kube.CatchUp(ctx, in.Entry.Informer, in.Entry.CRD.GVR(), ns, name); err != nil {
				result.Notes = append(result.Notes, fmt.Sprintf("after cycle %d: %v", cycle, err))
			}
		}

		// Steady: this pass repeats the last one, or writes nothing at all
		// (status a pass wrote is kept, so a settled CR needs no write).
		if !result.Steady && cycle > 1 && (noWrites(cycleResult.Ops) || opsMatch(cycleResult.Ops, prevCycleOps)) {
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
		Blueprint: deps.Blueprint,
		Clientset: deps.Kube.Clientset(),
		Namespace: &informer.NamespaceFilter{
			AllowedNamespaces:    []string(crd.AllAllowedNamespaces()),
			RestrictedNamespaces: []string(crd.AllRestrictedNamespaces()),
		},
	}, in.GVK, raw, nil)
}

// Op is one recorded operation of the fake client.
type Op = kubefake.Op
type Ops = kubefake.Ops

// compile-time check that *kubefake.Kubeclient satisfies loopKube.
var _ loopKube = (*kubefake.Kubeclient)(nil)

// compile-time check that cache.SharedIndexInformer is accessible from cache.
var _ cache.SharedIndexInformer = (*fakeInformer)(nil)
