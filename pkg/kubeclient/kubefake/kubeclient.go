package kubefake

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/intentd/inrun/pkg/util/kube"
	"strings"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	corev1 "k8s.io/api/core/v1"
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
	Resource    string // "deployments", "services", etc.
	Subresource string // "status", "custom", etc.
	Namespace   string
	Name        string
	At          time.Time
}

// fakeShared holds the mutable, mutex-guarded state shared across all
// copies of a Kubeclient.
type fakeShared struct {
	mu           sync.Mutex
	ops          []Op
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
	tracker       k8stesting.ObjectTracker // seeded objects, read by Get
}

// dynamicObjects seeds the fake dynamic client's tracker at construction —
// this must happen at construction, not via Tracker().Add() afterward,
// because fakedynamic.NewSimpleDynamicClient derives its GVR→ListKind
// mapping from the objects it's given; List() calls for a GVR that wasn't
// present at construction time fail even if the object is added later.
// Used to make an in-progress-reconcile CR (and any pre-existing sibling
// instances from a multi-doc CR file) visible to reconcile-time checks that
// list other instances of the same CRD, like unique: true.
func New(scheme *runtime.Scheme, dynamicObjects ...runtime.Object) *Kubeclient {
	f := &Kubeclient{shared: &fakeShared{}}

	cs := fake.NewClientset()
	// PrependReactor intercepts every operation and records it before the
	// default object-tracker reactor handles it. AddReactor appends to the
	// chain AFTER the tracker, so it is never reached — PrependReactor is
	// required here.
	cs.Fake.PrependReactor("*", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		f.shared.mu.Lock()
		f.shared.ops = append(f.shared.ops, Op{
			Cycle:     f.shared.currentCycle,
			Verb:      verbFromAction(action),
			Resource:  action.GetResource().Resource,
			Namespace: action.GetNamespace(),
			Name:      nameFromAction(action),
			At:        time.Now(),
		})
		f.shared.mu.Unlock()
		return false, nil, nil
	})

	f.clientset = cs
	f.scheme = scheme

	listKinds := ListKindsForSeeds(scheme, dynamicObjects) // Used to make available built resources

	dyn := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, dynamicObjects...)

	// PrependReactor intercepts every operation and records it before the
	// default object-tracker reactor handles it. AddReactor appends to the
	// chain AFTER the tracker, so it is never reached — PrependReactor is
	// required here.
	dyn.Fake.PrependReactor("*", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		f.shared.mu.Lock()
		f.shared.ops = append(f.shared.ops, Op{
			Cycle:     f.shared.currentCycle,
			Verb:      verbFromAction(action),
			Resource:  action.GetResource().Resource,
			Namespace: action.GetNamespace(),
			Name:      nameFromAction(action),
			At:        time.Now(),
		})
		f.shared.mu.Unlock()
		// The plain object tracker has no server-side apply; answer an apply
		// with the applied object, as an API server would.
		if pa, ok := action.(k8stesting.PatchAction); ok && pa.GetPatchType() == k8stypes.ApplyPatchType {
			u := &unstructured.Unstructured{}
			if err := u.UnmarshalJSON(pa.GetPatch()); err != nil {
				return true, nil, err
			}
			return true, u, nil
		}
		return false, nil, nil
	})
	f.dynamic = dyn
	f.tracker = dyn.Tracker()
	f.mapper = &fakeMapper{}

	return f
}

// Seed adds objects that exist before the operator starts (a CRD's setup
// files). Built-in kinds go to the typed clientset, so reads such as a
// remote reconciler's token Secret find them; everything else goes to the
// dynamic tracker.
func (f *Kubeclient) Seed(objs ...*unstructured.Unstructured) error {
	for _, u := range objs {
		gvk := u.GroupVersionKind()
		typed, err := clientgoscheme.Scheme.New(gvk)
		if err != nil {
			if err := f.tracker.Add(u); err != nil {
				return fmt.Errorf("seed %s %s: %w", gvk.Kind, u.GetName(), err)
			}
			continue
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, typed); err != nil {
			return fmt.Errorf("seed %s %s: %w", gvk.Kind, u.GetName(), err)
		}
		// The API server folds stringData into data; do the same.
		if sec, ok := typed.(*corev1.Secret); ok && len(sec.StringData) > 0 {
			if sec.Data == nil {
				sec.Data = map[string][]byte{}
			}
			for k, v := range sec.StringData {
				sec.Data[k] = []byte(v)
			}
			sec.StringData = nil
		}
		if err := f.clientset.(*fake.Clientset).Tracker().Add(typed); err != nil {
			return fmt.Errorf("seed %s %s: %w", gvk.Kind, u.GetName(), err)
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
func (f *Kubeclient) Ops() []Op {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	result := make([]Op, len(f.shared.ops))
	copy(result, f.shared.ops)
	return result
}

// OpsForCycle returns operations from one reconcile cycle.
func (f *Kubeclient) OpsForCycle(cycle int) []Op {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	var result []Op
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

// fakeMapper satisfies meta.RESTMapper for the simulation context.
// Returns a fixed mapping for standard resource types.
type fakeMapper struct{}

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
	if baseKind == "Namespace" || baseKind == "ClusterRole" || baseKind == "ClusterRoleBinding" {
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

// Patch methods record each operation and apply it the way the API server
// would, to the object and to the informer's copy the next cycle reads.

func (f *Kubeclient) PatchFinalizers(_ context.Context, obj runtime.Object, finalizers []string, _ metav1.PatchOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:    f.shared.currentCycle,
		Verb:     "patch",
		Resource: "finalizers",
		Name:     nameFromRuntimeObject(obj),
		At:       time.Now(),
	})
	f.shared.mu.Unlock()
	f.persistMeta(obj, func(mo metav1.Object) { mo.SetFinalizers(finalizers) })
	return nil
}

func (f *Kubeclient) PatchLabels(_ context.Context, obj runtime.Object, base, desired map[string]string, _ metav1.PatchOptions) error {
	if stringMapsEqual(base, desired) {
		return nil
	}
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:    f.shared.currentCycle,
		Verb:     "patch",
		Resource: "labels",
		Name:     nameFromRuntimeObject(obj),
		At:       time.Now(),
	})
	f.shared.mu.Unlock()
	f.persistMeta(obj, func(mo metav1.Object) { mo.SetLabels(desired) })
	return nil
}

func (f *Kubeclient) PatchAnnotations(_ context.Context, obj runtime.Object, annotations map[string]string, _ metav1.PatchOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:    f.shared.currentCycle,
		Verb:     "patch",
		Resource: "annotations",
		Name:     nameFromRuntimeObject(obj),
		At:       time.Now(),
	})
	f.shared.mu.Unlock()
	// A merge patch adds to the existing annotations; it does not replace them.
	f.persistMeta(obj, func(mo metav1.Object) {
		merged := mo.GetAnnotations()
		if merged == nil {
			merged = map[string]string{}
		}
		for k, v := range annotations {
			merged[k] = v
		}
		mo.SetAnnotations(merged)
	})
	return nil
}

func (f *Kubeclient) PatchStatus(_ context.Context, obj domain.Object, statusFields map[string]interface{}, _ metav1.PatchOptions) error {
	if len(statusFields) == 0 {
		return nil // the real client sends nothing either
	}
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:    f.shared.currentCycle,
		Verb:     "patch",
		Resource: "status",
		Name:     obj.GetName(),
		At:       time.Now(),
	})
	f.shared.mu.Unlock()
	f.persistMerge(obj, "status", statusFields)
	return nil
}

func (f *Kubeclient) PatchSpec(_ context.Context, obj domain.Object, specFields map[string]interface{}, _ metav1.PatchOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:    f.shared.currentCycle,
		Verb:     "patch",
		Resource: "spec",
		Name:     obj.GetName(),
		At:       time.Now(),
	})
	f.shared.mu.Unlock()
	f.persistMerge(obj, "spec", specFields)
	return nil
}

// persistMeta applies a metadata patch the way the API server would: to obj,
// and to the copy the informer holds, which the next cycle prepares from.
// Without it, maintain would find its finalizer and labels missing and patch
// them again on every cycle.
func (f *Kubeclient) persistMeta(obj runtime.Object, mutate func(metav1.Object)) {
	mo, ok := obj.(metav1.Object)
	if !ok {
		return
	}
	mutate(mo)
	f.updateStored(obj, func(raw interface{}) {
		if stored, ok := raw.(metav1.Object); ok {
			mutate(stored)
		}
	})
}

// persistMerge applies a JSON merge patch of {field: fields} the way the API
// server would: to obj, and to the informer's copy the next cycle prepares
// from. Without it, a status a pass wrote (an id the service assigned) is
// gone on the next pass, and anything that reads it back sees nothing.
func (f *Kubeclient) persistMerge(obj runtime.Object, field string, fields map[string]interface{}) {
	patch, err := json.Marshal(map[string]interface{}{field: fields})
	if err != nil {
		return
	}
	apply := func(o runtime.Object) {
		doc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
		if err != nil {
			return
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			return
		}
		merged, err := jsonpatch.MergePatch(raw, patch)
		if err != nil {
			return
		}
		var out map[string]interface{}
		if err := json.Unmarshal(merged, &out); err != nil {
			return
		}
		if u, ok := o.(*unstructured.Unstructured); ok {
			u.Object = out
			return
		}
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(out, o)
	}
	apply(obj)
	f.updateStored(obj, func(stored interface{}) {
		if o, ok := stored.(runtime.Object); ok {
			apply(o)
		}
	})
}

// updateStored applies mutate to the informer's copy of obj, when it holds
// one that is not obj itself, and stores it back.
func (f *Kubeclient) updateStored(obj runtime.Object, mutate func(interface{})) {
	mo, ok := obj.(metav1.Object)
	if !ok || f.informer == nil {
		return
	}
	key := mo.GetName()
	if ns := mo.GetNamespace(); ns != "" {
		key = ns + "/" + key
	}
	idx := f.informer.GetIndexer()
	raw, exists, err := idx.GetByKey(key)
	if err != nil || !exists {
		return
	}
	storedObj, ok := raw.(runtime.Object)
	if !ok || !sameKind(storedObj, obj) {
		return
	}
	if stored, ok := raw.(metav1.Object); !ok || stored == mo {
		return
	}
	mutate(raw)
	_ = idx.Update(raw)
}
