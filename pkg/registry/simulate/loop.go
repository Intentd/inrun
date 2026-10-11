package simulate

import (
	"context"
	"fmt"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/apply"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/event"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/post"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/prepare"
	"github.com/inrundev/inrun/pkg/template"
	apitypes "k8s.io/apimachinery/pkg/types"
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

// runLoop runs the reconciler for up to maxCycles and returns the aggregate
// result. Notes must be appended by the caller.
func runLoop(ctx context.Context, r domain.Reconciler, kube loopKube, key string, maxCycles int, prepInput prepare.Input) *Result {
	result := &Result{}
	var prevCycleOps []Op

	for cycle := 1; cycle <= maxCycles; cycle++ {
		kube.AdvanceCycle()

		cycleResult := CycleResult{Cycle: cycle}
		ns, name, _ := cache.SplitMetaNamespaceKey(key)

		prepared, _, err := prepare.Prepare(ctx, prepInput)
		if err != nil {
			cycleResult.Error = fmt.Errorf("prepare: %w", err)
			result.Cycles = append(result.Cycles, cycleResult)
			continue
		}
		if prepared == nil {
			// Object not found, deleted, or namespace-blocked — nothing to reconcile.
			result.Cycles = append(result.Cycles, cycleResult)
			continue
		}

		req := domain.Request{
			Key:            key,
			NamespacedName: apitypes.NamespacedName{Namespace: ns, Name: name},
			Prepared:       prepared,
		}
		req.Prepared.Args = prepare.Args(prepared)
		res, err := r.Reconcile(ctx, req)
		// Apply the Result's writes the way the coordinator worker does.
		if err == nil {
			err = apply.Result(ctx, kube, prepared.Object, prepInput.Entry.CRD.AllManagedResources(), res)
		}
		cycleResult.Error = err

		// Post-reconcile: status patch + emit — mirrors the coordinator worker.
		// Status patching lives in post.Apply; without this call the CR's status
		// fields would never be written and status-subresource assertions would fail.
		if resolver, ok := prepared.Context.(*template.Resolver); ok {
			box := prepare.BoxFrom(prepared)
			post.Apply(ctx, post.Input{
				CRD:      prepInput.Entry.CRD,
				Kube:     kube,
				Recorder: event.Discard(),
				Status:   res.Status,
			}, prepared.Object, resolver, box, cycleResult.Error, nil)
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

// compile-time check that *FakeKubeclient satisfies loopKube.
var _ loopKube = (*FakeKubeclient)(nil)

// compile-time check that cache.SharedIndexInformer is accessible from cache.
var _ cache.SharedIndexInformer = (*fakeInformer)(nil)
