package kubefake

import (
	"context"
	"fmt"
	"github.com/intentd/inrun/pkg/util/kube"
	"strings"
	"sync"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/kubeclient"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

// Op is one recorded cluster operation.
type Op struct {
	Cycle       int
	Verb        string // "create", "update", "delete", "get", "patch"
	Group       string // API group of Resource; "" for core and for writes recorded by method
	Version     string // API version of Resource; "" when unknown
	Resource    string // "deployments", "services", etc.
	Subresource string // "status", "custom", etc.
	Namespace   string
	Name        string
	At          time.Time
}

type Ops = []Op

// fakeShared holds the mutable, mutex-guarded state shared across all
// copies of a Kubeclient.
type fakeShared struct {
	mu           sync.Mutex
	ops          Ops
	currentCycle int
}

// Kubeclient implements kubeclient.Interface using k8s fakes.
// All operations are recorded in Ops() for simulation output.
type Kubeclient struct {
	clientset kubernetes.Interface
	dynamic   dynamic.Interface
	mapper    meta.RESTMapper
	scheme    *runtime.Scheme
	shared    *fakeShared

	informer      cache.SharedIndexInformer
	eventRecorder kubeclient.EventRecorder
	storeFor      func(schema.GroupVersionKind) cache.Store
	indexerFor    func(schema.GroupVersionKind) cache.Indexer
	forceConflict *bool
	tracker       k8stesting.ObjectTracker                                 // the custom tracker: every kind that is not built in
	store         k8stesting.ReactionFunc                                  // serves an action from the store (see store.go)
	defaults      map[schema.GroupVersionKind]func(map[string]interface{}) // per kind: the API server's schema defaulting (SetDefaults)
	scopes        map[schema.GroupKind]bool                                // per kind: namespaced or not (SetScope)
}

// SetScope registers whether gk is namespaced, as a CRD's spec.scope says;
// the REST mapper answers with it, and the store clears metadata.namespace
// on a cluster-scoped object, as the API server does.
func (f *Kubeclient) SetScope(gk schema.GroupKind, namespaced bool) {
	if f.scopes == nil {
		f.scopes = map[schema.GroupKind]bool{}
	}
	f.scopes[gk] = namespaced
}

// namespaced reports whether kind is namespaced: as registered, else as the
// built-in registry says, else true.
func (f *Kubeclient) namespaced(group, version, kind string) bool {
	if n, ok := f.scopes[schema.GroupKind{Group: group, Kind: kind}]; ok {
		return n
	}
	if b, ok := children.LookupBuiltInByGVK(group, version, kind); ok {
		return b.Namespaced
	}
	return true
}

// foldStringData moves a Secret's stringData into data, as the API server
// does on every write; it reports whether there was any.
func foldStringData(sec *corev1.Secret) bool {
	if len(sec.StringData) == 0 {
		return false
	}
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	for k, v := range sec.StringData {
		sec.Data[k] = []byte(v)
	}
	sec.StringData = nil
	return true
}

// New returns a fake cluster holding dynamicObjects: the CR under
// simulation and any pre-existing sibling instances from a multi-doc CR
// file, visible to reconcile-time checks that list other instances of the
// same CRD, like unique: true. Their list kinds are registered at
// construction, because the fake dynamic client can only List a resource
// whose list kind it was given then.
func New(scheme *runtime.Scheme, dynamicObjects ...runtime.Object) *Kubeclient {
	f := &Kubeclient{shared: &fakeShared{}, scheme: scheme}

	cs := fake.NewClientset()
	// PrependReactor intercepts every operation and records it before the
	// default object-tracker reactor handles it. AddReactor appends to the
	// chain AFTER the tracker, so it is never reached — PrependReactor is
	// required here.
	// Prepended in reverse: the recorder runs first, then a Secret write is
	// folded as the API server folds it (see foldSecrets).
	cs.Fake.PrependReactor("*", "secrets", foldSecrets(cs.Tracker()))
	cs.Fake.PrependReactor("*", "*", f.record)
	f.clientset = cs

	// Every kind that is not built in lives in the custom tracker (see
	// store.go); the dynamic client reads and writes both trackers.
	custom := newCustomTracker(scheme)
	for _, obj := range dynamicObjects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		tracker := custom
		if _, byGVK := builtins(); byGVK[gvk] != (schema.GroupVersionResource{}) {
			typed, err := typedFrom(obj, gvk)
			if err != nil {
				panic(fmt.Sprintf("kubefake: seeding %s: %v", gvk.Kind, err))
			}
			obj, tracker = typed, cs.Tracker()
		} else {
			f.registerKind(gvk)
		}
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Sprintf("kubefake: seeding %s: %v", gvk.Kind, err))
		}
	}

	listKinds := ListKindsForSeeds(scheme, dynamicObjects) // Used to make available built resources
	dyn := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	// Prepended in reverse: the recorder runs first, then the store
	// answers. The client's own tracker is never reached.
	f.store = f.storeReaction(cs.Tracker(), custom)
	dyn.Fake.PrependReactor("*", "*", f.store)
	dyn.Fake.PrependReactor("*", "*", f.record)
	f.dynamic = dyn
	f.tracker = custom
	f.mapper = &fakeMapper{f: f}

	return f
}

// record appends an action to the op log and leaves it to the next reactor.
func (f *Kubeclient) record(action k8stesting.Action) (bool, runtime.Object, error) {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      verbFromAction(action),
		Group:     action.GetResource().Group,
		Version:   action.GetResource().Version,
		Resource:  action.GetResource().Resource,
		Namespace: action.GetNamespace(),
		Name:      nameFromAction(action),
		At:        time.Now(),
	})
	f.shared.mu.Unlock()
	return false, nil, nil
}

// ListKindsForSeeds starts from the registry's full built-in list-kind map and
// merges the list kind for every seeded object's GVR. The registry covers
// built-ins a reconciler might List without seeding (e.g. ConfigMaps); the
// seeds cover the CRD under test and any siblings.
func ListKindsForSeeds(scheme *runtime.Scheme, seeds []runtime.Object) map[schema.GroupVersionResource]string {
	listKinds := children.ListKindsForFakeDynamic()

	for _, obj := range seeds {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			if kinds, _, err := scheme.ObjectKinds(obj); err == nil && len(kinds) > 0 {
				gvk = kinds[0]
			}
		}
		if gvk.Empty() {
			continue
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		listKinds[gvr] = gvk.Kind + "List"
	}

	return listKinds
}

// verbFromAction returns "apply" for Server-Side Apply patch operations,
// "patch" for all other patch types, and the raw verb for everything else.
func verbFromAction(action k8stesting.Action) string {
	if pa, ok := action.(k8stesting.PatchAction); ok {
		if pa.GetPatchType() == k8stypes.ApplyPatchType {
			return "apply"
		}
	}
	return action.GetVerb()
}

// nameFromAction extracts the resource name from a test action. Actions that
// name their object (get, delete, patch) satisfy GetAction; actions that carry
// it (create, update) satisfy CreateAction, so one case covers each.
func nameFromAction(action k8stesting.Action) string {
	switch a := action.(type) {
	case k8stesting.GetAction:
		return a.GetName()
	case k8stesting.CreateAction:
		if acc, ok := a.GetObject().(metav1.Object); ok {
			return acc.GetName()
		}
	}
	return ""
}

// Seed adds objects that exist before the operator starts (a CRD's setup
// files). Built-in kinds go to the typed clientset, so reads such as a
// remote reconciler's token Secret find them; everything else goes to the
// dynamic tracker.
func (c *Kubeclient) Seed(objs ...*unstructured.Unstructured) error {
	for _, u := range objs {
		gvk := u.GroupVersionKind()
		typed, err := clientgoscheme.Scheme.New(gvk)
		if err != nil {
			if err := c.seedToTracker(u); err != nil {
				return err
			}
			continue
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, typed); err != nil {
			return fmt.Errorf("seed %s %s: %w", gvk.Kind, u.GetName(), err)
		}
		if sec, ok := typed.(*corev1.Secret); ok {
			foldStringData(sec)
		}
		if err := c.seedToTracker(typed); err != nil {
			return err
		}
	}
	return nil
}

// resolveObjectMapping returns the GVK and REST mapping for obj.
// Unstructured objects carry the GVK directly; typed objects are looked up via the scheme.
func (f *Kubeclient) resolveObjectMapping(obj runtime.Object) (schema.GroupVersionKind, *meta.RESTMapping, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "" {
		gvks, _, err := clientgoscheme.Scheme.ObjectKinds(obj)
		if err != nil || len(gvks) == 0 {
			return schema.GroupVersionKind{}, nil, fmt.Errorf("seed: could not determine GVK for %T", obj)
		}
		gvk = gvks[0]
	}
	mapping, err := f.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionKind{}, nil, fmt.Errorf("seed %s: resolve GVR: %w", gvk.Kind, err)
	}
	return gvk, mapping, nil
}

// seedToTracker adds obj to the appropriate tracker, updating on AlreadyExists.
// Unstructured objects go to the dynamic tracker; typed objects go to the clientset tracker.
func (f *Kubeclient) seedToTracker(obj runtime.Object) error {
	gvk, mapping, err := f.resolveObjectMapping(obj)
	if err != nil {
		return err
	}
	acc, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("seed %s: accessor: %w", gvk.Kind, err)
	}
	var tracker k8stesting.ObjectTracker
	if _, ok := obj.(*unstructured.Unstructured); ok {
		tracker = f.tracker
		f.registerKind(gvk)
	} else {
		tracker = f.clientset.(*fake.Clientset).Tracker()
	}
	if err := tracker.Add(obj); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("seed %s %s: %w", gvk.Kind, acc.GetName(), err)
		}
		if err := tracker.Update(mapping.Resource, obj, acc.GetNamespace()); err != nil {
			return fmt.Errorf("seed %s %s: %w", gvk.Kind, acc.GetName(), err)
		}
	}
	return nil
}

func (f *Kubeclient) Clientset() kubernetes.Interface  { return f.clientset }
func (f *Kubeclient) DynamicClient() dynamic.Interface { return f.dynamic }
func (f *Kubeclient) RESTMapper() meta.RESTMapper      { return f.mapper }
func (f *Kubeclient) RestConfig() *rest.Config         { return nil }
func (f *Kubeclient) Scheme() *runtime.Scheme          { return f.scheme }

func (f *Kubeclient) WithInformer(inf cache.SharedIndexInformer) kubeclient.Interface {
	cp := *f
	cp.informer = inf
	return &cp
}

func (f *Kubeclient) WithEventRecorder(ev kubeclient.EventRecorder) kubeclient.Interface {
	cp := *f
	cp.eventRecorder = ev
	return &cp
}

func (f *Kubeclient) GetInformer() cache.SharedIndexInformer     { return f.informer }
func (f *Kubeclient) GetEventRecorder() kubeclient.EventRecorder { return f.eventRecorder }

func (f *Kubeclient) WithStoreFor(fn func(schema.GroupVersionKind) cache.Store) kubeclient.Interface {
	cp := *f
	cp.storeFor = fn
	return &cp
}

func (f *Kubeclient) GetStoreFor() func(schema.GroupVersionKind) cache.Store { return f.storeFor }

func (f *Kubeclient) WithIndexerFor(fn func(schema.GroupVersionKind) cache.Indexer) kubeclient.Interface {
	cp := *f
	cp.indexerFor = fn
	return &cp
}

func (f *Kubeclient) GetIndexerFor() func(schema.GroupVersionKind) cache.Indexer {
	return f.indexerFor
}

func (k *Kubeclient) ForceConflict() *bool { return new(bool) }
func (f *Kubeclient) WithForceConflict(forceConflict *bool) kubeclient.Interface {
	cp := *f
	cp.forceConflict = forceConflict
	return &cp
}

// AdvanceCycle increments the cycle counter. Call between simulated reconciles.
func (f *Kubeclient) AdvanceCycle() {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.currentCycle++
}

// Ops returns all recorded operations in order.
func (f *Kubeclient) Ops() Ops {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	result := make(Ops, len(f.shared.ops))
	copy(result, f.shared.ops)
	return result
}

// OpsForCycle returns operations from one reconcile cycle.
func (f *Kubeclient) OpsForCycle(cycle int) Ops {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	var result Ops
	for _, op := range f.shared.ops {
		if op.Cycle == cycle {
			result = append(result, op)
		}
	}
	return result
}

// MarkDeploymentReady advances simulated state — marks a Deployment as Available.
// Call after the first reconcile cycle to allow the reconciler to progress
// through "Deploying" → "Ready" state transitions.
func (f *Kubeclient) MarkDeploymentReady(namespace, name string) {
	// The fake clientset stores objects in an object tracker.
	// We patch the Deployment's status directly.
	dep, err := f.clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return
	}
	if dep.Spec.Replicas != nil {
		dep.Status.AvailableReplicas = *dep.Spec.Replicas
		dep.Status.ReadyReplicas = *dep.Spec.Replicas
		dep.Status.Replicas = *dep.Spec.Replicas
	}
	f.clientset.AppsV1().Deployments(namespace).UpdateStatus(
		context.Background(), dep, metav1.UpdateOptions{})
}

// fakeMapper satisfies meta.RESTMapper for the simulation context. A kind's
// scope comes from what SetScope registered (a Blueprint's CRDs), else the
// built-in registry, else namespaced — as a real mapper learns it from
// discovery, so a client sends a cluster-scoped kind's request cluster-wide.
type fakeMapper struct{ f *Kubeclient }

func (m *fakeMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	version := "v1"
	if len(versions) > 0 && versions[0] != "" {
		version = versions[0]
	}

	// A List GVK (e.g. ConfigMapList) maps to the base resource (configmaps).
	// RESTMapper resolves the resource type, not the list variant; the list
	// kind is a client-side concept.
	baseKind := strings.TrimSuffix(gk.Kind, "List")

	scope := meta.RESTScopeNameNamespace
	if !m.f.namespaced(gk.Group, version, baseKind) {
		scope = meta.RESTScopeNameRoot
	}
	return &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    gk.Group,
			Version:  version,
			Resource: kube.PluralForKind(baseKind),
		},
		GroupVersionKind: gk.WithVersion(version),
		Scope:            fakescope(scope),
	}, nil
}

func (m *fakeMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (m *fakeMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}
func (m *fakeMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return input, nil
}
func (m *fakeMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return []schema.GroupVersionResource{input}, nil
}
func (m *fakeMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	mapping, err := m.RESTMapping(gk, versions...)
	if err != nil {
		return nil, err
	}
	return []*meta.RESTMapping{mapping}, nil
}
func (m *fakeMapper) ResourceSingularizer(resource string) (string, error) {
	return strings.TrimSuffix(resource, "s"), nil
}

type fakescope string

func (f fakescope) Name() meta.RESTScopeName { return meta.RESTScopeName(f) }
func (f fakescope) String() string           { return string(f) }

// Compile check — *Kubeclient must satisfy kubeclient.Interface.
var _ kubeclient.Interface = (*Kubeclient)(nil)

// The Patch* methods send what the real client sends — the body its
// builders make (kubeclient.FinalizersPatch, ...) — and client-go's tracker
// applies it to the store, as the API server applies it. Like the real
// client, they leave obj as it is: the informer's copy catches up with the
// store after the cycle (CatchUp).

func (f *Kubeclient) PatchFinalizers(_ context.Context, obj runtime.Object, finalizers []string, _ metav1.PatchOptions) error {
	p, err := kubeclient.FinalizersPatch(finalizers)
	if err != nil {
		return err
	}
	return f.sendPart(obj, p)
}

func (f *Kubeclient) PatchLabels(_ context.Context, obj runtime.Object, base, desired map[string]string, _ metav1.PatchOptions) error {
	p, changed, err := kubeclient.LabelsPatch(base, desired)
	if err != nil || !changed {
		return err
	}
	return f.sendPart(obj, p)
}

func (f *Kubeclient) PatchAnnotations(_ context.Context, obj runtime.Object, annotations map[string]string, _ metav1.PatchOptions) error {
	p, err := kubeclient.AnnotationsPatch(annotations)
	if err != nil {
		return err
	}
	return f.sendPart(obj, p)
}

func (f *Kubeclient) PatchStatus(_ context.Context, obj domain.Object, statusFields map[string]interface{}, _ metav1.PatchOptions) error {
	p, changed, err := kubeclient.StatusPatch(statusFields)
	if err != nil || !changed {
		return err
	}
	return f.sendPart(obj, p)
}

func (f *Kubeclient) PatchSpec(_ context.Context, obj domain.Object, specFields map[string]interface{}, _ metav1.PatchOptions) error {
	p, err := kubeclient.SpecPatch(specFields)
	if err != nil {
		return err
	}
	return f.sendPart(obj, p)
}

// sendPart records p under its part's name, as the envtest recorder names
// the same request, with obj's group and version, and applies it to the
// store. It hands the patch to the store directly, not through the dynamic
// client, so the op is recorded once.
func (f *Kubeclient) sendPart(obj runtime.Object, p kubeclient.PartPatch) error {
	gvr, _, err := f.resourceOf(obj)
	if err != nil {
		return err
	}
	acc, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "patch",
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  p.Part,
		Namespace: acc.GetNamespace(),
		Name:      acc.GetName(),
		At:        time.Now(),
	})
	f.shared.mu.Unlock()

	var subresources []string
	if p.Subresource != "" {
		subresources = []string{p.Subresource}
	}
	action := k8stesting.NewPatchSubresourceAction(gvr, acc.GetNamespace(), acc.GetName(), k8stypes.MergePatchType, p.Body, subresources...)
	_, _, err = f.store(action)
	return err
}
