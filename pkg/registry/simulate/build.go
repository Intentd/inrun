package simulate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kel"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// newFakeInformer wraps a static indexer as a SharedIndexInformer.
// The reconciler only calls GetIndexer() — all other methods are no-ops.
type fakeInformer struct {
	indexer cache.Indexer
	cache.SharedIndexInformer
}

func newFakeInformer(obj interface{}) (cache.SharedIndexInformer, error) {
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := idx.Add(obj); err != nil {
		return nil, fmt.Errorf("adding object to indexer: %w", err)
	}
	return &fakeInformer{indexer: idx}, nil
}

func (f *fakeInformer) GetIndexer() cache.Indexer { return f.indexer }

// buildPeerRegistry creates a coordinator registry populated with fake informers
// for each peer CRD that has a matching entry in peers. Peers not present in the
// map are skipped — only supplied CRs are visible to cross: declarations.
func buildPeerRegistry(bp *blueprint.Blueprint, selfName string, peers map[string]*unstructured.Unstructured) (*coordinator.ResourceBlueprint, error) {
	reg := coordinator.NewCoordinatorRegistry()
	for _, peerName := range bp.CRDNames() {
		if peerName == selfName {
			continue
		}
		peerEntry, ok := bp.CRDEntry(peerName)
		if !ok {
			continue
		}
		peerCR, ok := peers[strings.ToLower(peerEntry.APITypes.Kind)]
		if !ok {
			continue
		}
		peerInf, err := newFakeInformer(peerCR)
		if err != nil {
			return nil, fmt.Errorf("building peer informer for %s: %w", peerEntry.Name, err)
		}
		reg.Register(peerEntry, peerInf, nil)
	}
	return reg, nil
}

type fakeCluster struct {
	kube    *kubefake.Kubeclient
	seedObj interface{}
}

// buildFakeCluster creates a fake kubeclient seeded with the CR under test,
// any pre-existing same-kind instances, and the CRD's setup objects.
// It also resolves seedObj to the concrete Go type for typed CRDs so that
// constructor type-assertions and BindToObjectHooks closures succeed.
func buildFakeCluster(
	scheme *runtime.Scheme,
	cr *unstructured.Unstructured,
	crdEntry *types.CRDEntry,
	existingInstances []*unstructured.Unstructured,
	setup []string,
	crds crdFacts,
) (*fakeCluster, error) {
	// Seed the CR as the API server holds it once created (see created),
	// on copies so the caller's objects are untouched.
	cr = created(cr, crds.Defaults)
	seeded := make([]*unstructured.Unstructured, len(existingInstances))
	for i, existing := range existingInstances {
		seeded[i] = created(existing, crds.Defaults)
	}
	existingInstances = seeded

	// cr itself is included so the checker's self-exclusion by namespace/name
	// is exercised against real data alongside any pre-existing instances.
	dynamicObjects := make([]runtime.Object, 0, 1+len(existingInstances))
	dynamicObjects = append(dynamicObjects, cr)
	for _, existing := range existingInstances {
		dynamicObjects = append(dynamicObjects, existing)
	}
	kube := kubefake.New(scheme, dynamicObjects...)
	for gvk, d := range crds.Defaults {
		kube.SetDefaults(gvk, d)
	}
	for gk, namespaced := range crds.Scopes {
		kube.SetScope(gk, namespaced)
	}
	objs, err := setupObjects(crdEntry, setup)
	if err != nil {
		return nil, err
	}
	if err := kube.Seed(objs...); err != nil {
		return nil, err
	}

	// For typed CRDs, the indexer must hold the concrete Go type — not
	// *unstructured.Unstructured — so that constructor type-assertions
	// (raw.(*MyType)) and hook BindToObjectHooks closures succeed.
	seedObj := interface{}(cr)
	if objFactory, ok := types.ObjectRegistry[crdEntry.GVK()]; ok {
		if _, isObj := objFactory().(domain.Object); isObj {
			newObj := func() domain.Object { return objFactory().(domain.Object) }
			if typed, err := domain.ToTypedWith(domain.Object(cr), newObj); err == nil {
				seedObj = typed
			}
		}
	}
	return &fakeCluster{kube: kube, seedObj: seedObj}, nil
}

// reconcileRun holds the parts that differ between fake and envtest harnesses.
// recDeps is passed directly to reconcilers.Factory; its Kube/Informer fields
// differ between the two modes (envtest also sets StoreFor/IndexerFor).
// reconcileKube is used for reconcile.Deps.Kube — in the fake harness this is
// the client with the informer attached; in envtest it is the real client.
// runKube is used for runLoop — always the base client (fake or real).
type reconcileRun struct {
	recDeps       reconcilers.Deps
	reconcileKube kubeclient.Interface
	runKube       loopKube
	informer      cache.SharedIndexInformer
	peerReg       *coordinator.ResourceBlueprint
}

// runReconcileLoop builds the reconciler, resolves the cache key, and runs the
// reconcile loop. It is the shared tail of both harness modes.
func runReconcileLoop(ctx context.Context, crdEntry types.CRDEntry, run reconcileRun, bp *blueprint.Blueprint, cr *unstructured.Unstructured, result *Result, maxCycles int) (*Result, error) {
	recFactory, err := reconcilers.Factory(crdEntry, run.recDeps)
	if err != nil {
		return nil, err
	}
	r := recFactory()
	if crdEntry.WithRemoteDecl() || crdEntry.HasTargetRemoteDeclarations() {
		result.Notes = append(result.Notes, remoteNote)
	}

	key, err := resolveCacheKey(cr)
	if err != nil {
		return nil, err
	}

	deps := reconcile.Deps{
		Kube:      run.reconcileKube,
		Event:     event.Discard(),
		Blueprint: bp,
		Registry:  run.peerReg,
	}
	input := reconcile.Input{
		GVK:        crdEntry.GVKString(),
		Key:        key,
		Entry:      contract.RegistryEntry{CRD: crdEntry, Informer: run.informer},
		Reconciler: r,
		Health:     vitals.NewCRDHealth(crdEntry.Name),
	}

	loopResult := runLoop(ctx, deps, input, run.runKube, maxCycles)
	loopResult.Notes = append(result.Notes, loopResult.Notes...)
	loopResult.Startup = result.Startup
	loopResult.RandomSecretKeys = randomSecretKeys(crdEntry.Box(), bp.UserFunctions())
	loopResult.Final = finalObjects(ctx, run.runKube, crdEntry.GVR(), cr, loopResult.AllOps)
	return loopResult, nil
}

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

// quietLogs discards the reconciler's logs, controller-runtime's included,
// unless opts.Verbose: the simulation output is structured by the caller. It
// returns the function that restores them.
func quietLogs(opts RunOptions) (restore func()) {
	if opts.Verbose {
		return func() {}
	}
	prev := log.Logger
	log.Logger = *logger.Nop()
	return func() { log.Logger = prev }
}

// stubExternal answers external: HTTP calls with an empty 200 when
// opts.SkipExternal is set, and returns the function that undoes it. Without
// it, external calls hit the real network — useful when targeting a local mock
// server or a staging API from a development machine.
func stubExternal(opts RunOptions) (restore func()) {
	if !opts.SkipExternal {
		return func() {}
	}
	prev := external.HTTPTransport
	external.HTTPTransport = noopTransport{}
	return func() { external.HTTPTransport = prev }
}

// created returns a copy of u as the API server stores it on create: with
// its CRD's schema defaults applied, and what the server assigns where u
// sets none — a uid, a creation time, and generation 1. The reconciler reads
// them back: a defaulted field is there to read, the uid names the CR in its
// children's owner references and labels, the generation becomes
// observedGeneration.
func created(u *unstructured.Unstructured, defaults map[schema.GroupVersionKind]func(map[string]interface{})) *unstructured.Unstructured {
	cp := u.DeepCopy()
	if d := defaults[cp.GroupVersionKind()]; d != nil {
		d(cp.Object)
	}
	if cp.GetUID() == "" {
		cp.SetUID(uuid.NewUUID())
	}
	if ts := cp.GetCreationTimestamp(); ts.IsZero() {
		cp.SetCreationTimestamp(metav1.Now())
	}
	if cp.GetGeneration() == 0 {
		cp.SetGeneration(1)
	}
	return cp
}

// randomSecretKeys returns the data keys of crd's declared Secrets whose
// value is a template calling one of KEL's random functions
// (randomAlphanumeric, randomHex, ... — kel's "random" domain), directly or
// through a function the Blueprint defines. Two runs generate different
// values for them by design, so --compare reports a difference there as
// varying, not as a difference.
func randomSecretKeys(box *types.OperatorBoxConfig, functions types.KELFunctionRegistry) []string {
	if box == nil {
		return nil
	}
	var random []string
	for _, f := range kel.Builtin().ByDomain("random") {
		random = append(random, f.Name)
	}
	keys := map[string]bool{}
	for _, ht := range []*types.HookTemplates{box.EffectiveOnCreate(), box.EffectiveOnReconcile()} {
		if ht == nil {
			continue
		}
		for _, sec := range ht.Secrets {
			for key, value := range sec.Data {
				if types.IsTemplate(value) && functions.Calls(value, random...) {
					keys[key] = true
				}
			}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
