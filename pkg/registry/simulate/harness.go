package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/event"
	"github.com/inrundev/inrun/pkg/external"
	"github.com/inrundev/inrun/pkg/intent"
	"github.com/inrundev/inrun/pkg/runtime/coordinator"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/contract"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/prepare"
	"github.com/inrundev/inrun/pkg/runtime/reconcilers/generic"
	"github.com/inrundev/inrun/pkg/types"
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

	// Peers holds CRs for sibling CRDs in the same Catalog, keyed by lowercase kind.
	// When set, cross: declarations in the reconciler can observe these CRs
	// via the fake catalog registry instead of returning empty results.
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
// cat is the parsed Catalog.
// crdName is the CRD entry to simulate.
// cr is the CR to reconcile.
// maxCycles is the maximum number of reconcile cycles.
func Run(ctx context.Context, cat *catalog.Catalog, crdName string, cr *unstructured.Unstructured, maxCycles int, opts RunOptions) (*Result, error) {
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

	crdEntry, ok := cat.CRDEntry(crdName)
	if !ok {
		return nil, fmt.Errorf("CRD %q not found in Catalog", crdName)
	}
	if opts.Target != "" {
		// As if created through that target, so the reconcile resolves it.
		cr = cr.DeepCopy()
		intent.SetTarget(cr, opts.Target)
	}
	// Strip cross-namespace copy resources (fromNamespace / toNamespaces) from
	// all hook phases before the fake reconciler runs. These require a live API
	// server to read the source object; in simulation they would error and block
	// all subsequent resources in the same cycle.
	//
	// The removed resources are surfaced as notes in the result so the simulate
	// output explains exactly what was omitted and why.
	result := &Result{}
	box := effectiveOperatorBox(crdEntry, cr, opts.Target)
	// Copy the effective box so we don't mutate the original CRD entry.
	boxCopy := *box
	if boxCopy.Reconcile != nil {
		reconcileCopy := *boxCopy.Reconcile
		for _, phase := range []*types.HookTemplates{
			reconcileCopy.OnCreate,
			reconcileCopy.OnReconcile,
			reconcileCopy.OnDelete,
		} {
			if phase == nil {
				continue
			}
			filtered, skipped := types.FilterSimulatable(*phase)
			*phase = filtered
			result.Notes = append(result.Notes, skipped...)
		}
		boxCopy.Reconcile = &reconcileCopy
	}
	// Build effective CRD entry — reconciler uses target's operatorBox, not the CRD-level one.
	effectiveCRDEntry := crdEntry
	effectiveCRDEntry.OperatorBox = &boxCopy

	scheme, err := cat.Scheme()
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
	fakeKube := NewFakeKubeclient(scheme, dynamicObjects...)
	setup, err := setupObjects(&crdEntry)
	if err != nil {
		return nil, err
	}
	if err := fakeKube.Seed(setup...); err != nil {
		return nil, err
	}

	// Pre-seed managed labels/annotations so the reconciler's idempotency
	// guards skip those patches on every cycle.
	seedManagedMeta(cr, cat.Metadata().Name)

	// For typed CRDs, the indexer must hold the concrete Go type — not
	// *unstructured.Unstructured — so that constructor type-assertions
	// (raw.(*MyType)) and hook BindToObjectHooks closures succeed.
	// Convert via JSON round-trip: unstructured.Object → JSON → typed struct.
	seedObj := interface{}(cr)
	newObjFn := func() domain.Object { return &unstructured.Unstructured{} }

	if objFactory, ok := types.ObjectRegistry[gvk]; ok {
		typed := objFactory()
		if jsonBytes, err := json.Marshal(cr.Object); err == nil {
			if json.Unmarshal(jsonBytes, typed) == nil {
				if domObj, ok := typed.(domain.Object); ok {
					seedObj = domObj
					newObjFn = func() domain.Object {
						return types.ObjectRegistry[gvk]().(domain.Object)
					}
				}
			}
		}
	}

	// Build a fake informer backed by a static indexer containing the CR.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(seedObj); err != nil {
		return nil, fmt.Errorf("adding CR to indexer: %w", err)
	}
	informer := newFakeInformer(indexer)

	// Look up hooks from the registry. In the standard inrun binary the map is
	// empty for custom types and hookBinder stays nil.
	// In a custom operator binary produced by `make registry && make build`,
	// the init() in zz_generated_typeregistry.go populates HookRegistry so
	// the actual hook function runs inside the fake cluster.
	var hookBinder domain.AnyReconcileHooks
	if fn, ok := types.HookRegistry[gvk]; ok {
		hookBinder = fn()
	}

	// Build a peer registry so cross: declarations can read sibling CRDs' CRs
	// from the fake informer cache rather than returning empty results.
	// Each peer CR is seeded into its own static indexer and wrapped in a
	// fakeInformer; the registry maps CRD name → informer for readCross().
	peerRegistry := coordinator.NewCoordinatorRegistry()
	for _, peerName := range cat.CRDNames() {
		if peerName == crdEntry.Name {
			continue
		}
		peerEntry, ok := cat.CRDEntry(peerName)
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

	// Build the reconciler. Constructor path: use it directly with the fake
	// kubeclient and a discarding event recorder.
	// Fallback: generic.Reconciler with the typed newObj factory and peer registry
	// so hook BindToObjectHooks type-assertions and cross: lookups both work.
	var r domain.Reconciler
	if crdEntry.WithRemoteDecl() {
		r, err = newRemoteReconciler(&crdEntry, fakeKube.WithInformer(informer), outsidePodNamespace)
		if err != nil {
			return nil, err
		}
		result.Notes = append(result.Notes, remoteNote)
	} else if factoryFn, ok := types.ReconcilerRegistry[gvk]; ok {
		r = factoryFn(fakeKube.WithInformer(informer).WithEventRecorder(event.Discard()))
	} else {
		r = generic.New(
			effectiveCRDEntry,
			informer,
			nil,
			fakeKube,
			hookBinder,
			newObjFn,
			cat,
		)
	}

	key, err := resolveCacheKey(cr)
	if err != nil {
		return nil, err
	}

	r = wrapWithGate(r, boxCopy.PreReconcile, cat.Notes, getFromIndexerOrFallback(indexer, key, cr))

	prepInput := prepare.Input{
		Entry: contract.RegistryEntry{
			CRD:      effectiveCRDEntry,
			Informer: informer,
		},
		Key:      key,
		Cat:      cat,
		Kube:     fakeKube,
		Registry: peerRegistry,
	}

	loopResult := runLoop(ctx, r, fakeKube, key, maxCycles, prepInput)
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
