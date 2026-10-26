package simulate

import (
	"context"
	"fmt"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"io"
	"net/http"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
	"github.com/intentd/inrun/pkg/types"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// noopTransport returns an empty 200 response for every request.
// Used to stub out external: HTTP calls during simulation so they
// execute the full call path but produce empty result fields
// rather than real network errors.
type noopTransport struct{}

func (noopTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// Result is the output of one simulation run.
type Result struct {
	Cycles   []CycleResult
	Steady   bool
	SteadyAt int      // cycle number where steady state was first detected (0 if not reached)
	Notes    []string // informational notes about blocks that could not execute (e.g. constructor body)
	AllOps   []Op     // every op recorded across all cycles, for diagnostic use
}

// CycleResult is the output of one reconcile cycle.
type CycleResult struct {
	Cycle int
	Ops   []Op
	Error error
}

// RunOptions controls optional behaviour for a simulation run.
type RunOptions struct {
	// SkipExternal stubs all external: HTTP calls with an empty 200 response.
	// When false (the default), external calls are attempted against the real network.
	SkipExternal bool

	// Peers holds CRs for sibling CRDs in the same Blueprint, keyed by lowercase kind.
	// When set, cross: declarations in the reconciler can observe these CRs
	// via the fake blueprint registry instead of returning empty results.
	Peers map[string]*unstructured.Unstructured

	// ExistingInstances holds other, already-existing instances of the SAME
	// CRD being simulated — from extra documents of the CR's own kind in a
	// multi-doc CR file. They are seeded into the fake dynamic client (so
	// reconcile-time checks that list other instances, like operator:
	// unique, can see them) but are never themselves reconciled.
	ExistingInstances []*unstructured.Unstructured

	// Target specifies the target to use for this simulation
	Target string
}

// Run simulates the operator against an in-memory cluster.
//
// bp is the parsed Blueprint.
// crdName is the CRD entry to simulate.
// cr is the CR to reconcile.
// maxCycles is the maximum number of reconcile cycles.
func Run(ctx context.Context, bp *blueprint.Blueprint, crdName string, cr *unstructured.Unstructured, maxCycles int, opts RunOptions) (*Result, error) {
	// Silence the reconciler's JSON logs — simulation output is structured by the caller.
	prev := log.Logger
	log.Logger = log.Output(io.Discard)
	defer func() { log.Logger = prev }()

	// Stub out external: HTTP calls only when --skip-external is requested.
	// Without it, external calls hit the real network — useful when targeting a
	// local mock server or a staging API from a development machine.
	if opts.SkipExternal {
		prevTransport := external.HTTPTransport
		external.HTTPTransport = noopTransport{}
		defer func() { external.HTTPTransport = prevTransport }()
	}

	crdEntry, ok := bp.CRDEntry(crdName)
	if !ok {
		return nil, fmt.Errorf("CRD %q not found in Blueprint", crdName)
	}
	if opts.Target != "" {
		// As if created through that target, so the reconcile resolves it.
		cr = cr.DeepCopy()
		intent.SetTarget(cr, opts.Target)
	}
	// Templates that need a live API server (cross-namespace copies) are left
	// out of a copy of the entry, so the Blueprint's own is untouched; the notes
	// say what was omitted and why.
	result := &Result{}
	crdEntry, result.Notes = simulatable(crdEntry)

	scheme, err := bp.Scheme()
	if err != nil {
		return nil, fmt.Errorf("building scheme: %w", err)
	}

	gvk := schema.GroupVersionKind{
		Group:   crdEntry.APITypes.Group,
		Version: crdEntry.APITypes.Version,
		Kind:    crdEntry.APITypes.Kind,
	}

	// Build the fake cluster — seed the CR under test plus any pre-existing
	// same-kind instances (opts.ExistingInstances, from extra documents of
	// this CRD's own kind in a multi-doc CR file) into the dynamic client so
	// reconcile-time checks that list other instances, like operator:
	// unique, see real data instead of an empty list. cr itself is included
	// (not just ExistingInstances) so the checker's self-exclusion by
	// namespace/name is exercised against real data too.
	dynamicObjects := make([]runtime.Object, 0, 1+len(opts.ExistingInstances))
	dynamicObjects = append(dynamicObjects, cr)
	for _, existing := range opts.ExistingInstances {
		dynamicObjects = append(dynamicObjects, existing)
	}
	fakeKube := kubefake.New(scheme, dynamicObjects...)
	objs, err := setupObjects(&crdEntry)
	if err != nil {
		return nil, err
	}
	if err := fakeKube.Seed(objs...); err != nil {
		return nil, err
	}

	// For typed CRDs, the indexer must hold the concrete Go type — not
	// *unstructured.Unstructured — so that constructor type-assertions
	// (raw.(*MyType)) and hook BindToObjectHooks closures succeed.
	seedObj := interface{}(cr)

	if objFactory, ok := types.ObjectRegistry[gvk]; ok {
		if _, isObj := objFactory().(domain.Object); isObj {
			newObj := func() domain.Object { return objFactory().(domain.Object) }
			if typed, err := domain.ToTypedWith(domain.Object(cr), newObj); err == nil {
				seedObj = typed
			}
		}
	}

	// Build a fake informer backed by a static indexer containing the CR.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(seedObj); err != nil {
		return nil, fmt.Errorf("adding CR to indexer: %w", err)
	}
	informer := newFakeInformer(indexer)

	// Build a peer registry so cross: declarations can read sibling CRDs' CRs
	// from the fake informer cache rather than returning empty results.
	// Each peer CR is seeded into its own static indexer and wrapped in a
	// fakeInformer; the registry maps CRD name → informer for readCross().
	peerRegistry := coordinator.NewCoordinatorRegistry()
	for _, peerName := range bp.CRDNames() {
		if peerName == crdEntry.Name {
			continue
		}
		peerEntry, ok := bp.CRDEntry(peerName)
		if !ok {
			continue
		}
		peerCR, ok := opts.Peers[strings.ToLower(peerEntry.APITypes.Kind)]
		if !ok {
			continue
		}
		peerIdx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		_ = peerIdx.Add(peerCR)
		peerInf := newFakeInformer(peerIdx)
		peerGVKStr := schema.GroupVersionKind{
			Group:   peerEntry.APITypes.Group,
			Version: peerEntry.APITypes.Version,
			Kind:    peerEntry.APITypes.Kind,
		}.String()
		peerRegistry.Register(peerGVKStr, peerEntry, peerInf, nil)
	}

	// The reconciler the runtime would build for this CRD, over the fake cluster.
	factory, err := reconcilers.Factory(crdEntry, reconcilers.Deps{
		Kube:         fakeKube,
		Informer:     informer,
		Event:        event.Discard(),
		Blueprint:    bp,
		OwnNamespace: outsidePodNamespace,
	})
	if err != nil {
		return nil, err
	}
	r := factory()
	if crdEntry.WithRemoteDecl() || crdEntry.HasTargetRemoteDeclarations() {
		result.Notes = append(result.Notes, remoteNote)
	}

	key, err := resolveCacheKey(cr)
	if err != nil {
		return nil, err
	}

	deps := reconcile.Deps{
		Kube:      fakeKube.WithInformer(informer),
		Event:     event.Discard(),
		Blueprint: bp,
		Registry:  peerRegistry,
	}
	input := reconcile.Input{
		GVK:        crdEntry.GVKString(),
		Key:        key,
		Entry:      contract.RegistryEntry{CRD: crdEntry, Informer: informer},
		Reconciler: r,
		Health:     vitals.NewCRDHealth(crdEntry.Name),
	}

	// The same pass the runtime runs. The fake kubeclient carries the informer
	// so maintain's finalizer, label and annotation patches land on the cached
	// CR the next cycle prepares from, as they would through the API server.
	loopResult := runLoop(ctx, deps, input, fakeKube, maxCycles)
	loopResult.Notes = result.Notes
	return loopResult, nil
}

// newFakeInformer wraps a static indexer as a SharedIndexInformer.
// The reconciler only calls GetIndexer() — all other methods are no-ops.
type fakeInformer struct {
	indexer cache.Indexer
	cache.SharedIndexInformer
}

func newFakeInformer(idx cache.Indexer) cache.SharedIndexInformer {
	return &fakeInformer{indexer: idx}
}

func (f *fakeInformer) GetIndexer() cache.Indexer { return f.indexer }
