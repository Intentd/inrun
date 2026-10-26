package kubefake

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/managedfields"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
)

// Where the fake cluster keeps objects. One API server serves the typed and
// the dynamic client from one store, so a Deployment applied through one is
// read back through the other. The fake does the same with two trackers,
// one per kind of object, each shared by both clients:
//
//   - built-in kinds (client-go's scheme) live in the typed clientset's
//     tracker;
//   - every other kind lives in the custom tracker.
//
// Both are field-managed, so a server-side apply is stored and merged as
// the API server would: by field owner, with managedFields.

// builtins maps each built-in resource to its kind and back, from client-go's
// scheme: every kind that is an object and has a list.
var builtins = sync.OnceValues(func() (map[schema.GroupVersionResource]schema.GroupVersionKind, map[schema.GroupVersionKind]schema.GroupVersionResource) {
	byGVR := map[schema.GroupVersionResource]schema.GroupVersionKind{}
	byGVK := map[schema.GroupVersionKind]schema.GroupVersionResource{}
	known := clientgoscheme.Scheme.AllKnownTypes()
	for gvk := range known {
		if strings.HasSuffix(gvk.Kind, "List") || gvk.Version == runtime.APIVersionInternal {
			continue
		}
		if _, ok := known[gvk.GroupVersion().WithKind(gvk.Kind+"List")]; !ok {
			continue
		}
		obj, err := clientgoscheme.Scheme.New(gvk)
		if err != nil {
			continue
		}
		if _, err := meta.Accessor(obj); err != nil {
			continue
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		byGVR[gvr] = gvk
		byGVK[gvk] = gvr
	}
	return byGVR, byGVK
})

// builtinKind returns the kind of gvr when it is a built-in resource.
func builtinKind(gvr schema.GroupVersionResource) (schema.GroupVersionKind, bool) {
	byGVR, _ := builtins()
	gvk, ok := byGVR[gvr]
	return gvk, ok
}

// newCustomTracker returns the tracker for every kind that is not built in.
// Its kinds are deduced from the objects, as for any CRD without a schema
// at hand.
func newCustomTracker(scheme *runtime.Scheme) k8stesting.ObjectTracker {
	return k8stesting.NewFieldManagedObjectTracker(scheme,
		serializer.NewCodecFactory(scheme).UniversalDecoder(), managedfields.NewDeducedTypeConverter())
}

// storeReaction serves every action of the fake dynamic client from the
// tracker its resource lives in. A built-in object written through the
// dynamic client is converted to its Go type first, so the typed client
// reads it back as one.
func (f *Kubeclient) storeReaction(typed, custom k8stesting.ObjectTracker) k8stesting.ReactionFunc {
	typedReact, customReact := k8stesting.ObjectReaction(typed), k8stesting.ObjectReaction(custom)
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		if gvk, ok := builtinKind(action.GetResource()); ok {
			a, err := withTypedObject(action, gvk)
			if err != nil {
				return true, nil, err
			}
			handled, obj, err := typedReact(a)
			if err != nil || obj == nil {
				return handled, obj, err
			}
			if isWriteAction(action) {
				obj = foldStoredSecret(typed, action, obj)
			}
			u, err := unstructuredFromTyped(obj)
			return handled, u, err
		}
		gvk := kindOfWrite(action)
		if gvk.Empty() {
			// A patch body need not say its kind: take the stored object's.
			if obj, err := custom.Get(action.GetResource(), action.GetNamespace(), nameFromAction(action)); err == nil {
				gvk = obj.GetObjectKind().GroupVersionKind()
			}
		}
		f.registerKind(gvk)
		if gvk.Kind != "" && !f.namespaced(gvk.Group, gvk.Version, gvk.Kind) {
			action = withoutNamespace(action)
		}
		handled, obj, err := customReact(action)
		if err != nil || !isWriteAction(action) {
			return handled, obj, err
		}
		return handled, f.applyDefaults(custom, action, gvk, obj), nil
	}
}

// SetDefaults registers the defaulting the API server applies to objects of
// gvk: a CRD's structural schema defaults. Every write of gvk to the store
// is defaulted after it is applied, as the API server defaults a request
// before it stores it.
func (f *Kubeclient) SetDefaults(gvk schema.GroupVersionKind, defaults func(obj map[string]interface{})) {
	if f.defaults == nil {
		f.defaults = map[schema.GroupVersionKind]func(map[string]interface{}){}
	}
	f.defaults[gvk] = defaults
}

func isWriteAction(action k8stesting.Action) bool {
	switch action.GetVerb() {
	case "create", "update", "patch":
		return true
	}
	return false
}

// applyDefaults defaults the object a write left in the custom tracker,
// stores it back when that changed it, and returns what the write should
// answer. Nothing happens for a kind with no defaults.
func (f *Kubeclient) applyDefaults(custom k8stesting.ObjectTracker, action k8stesting.Action, gvk schema.GroupVersionKind, written runtime.Object) runtime.Object {
	defaults := f.defaults[gvk]
	if defaults == nil {
		return written
	}
	name := nameFromAction(action)
	stored, err := custom.Get(action.GetResource(), action.GetNamespace(), name)
	if err != nil {
		return written
	}
	u, ok := stored.(*unstructured.Unstructured)
	if !ok {
		return written
	}
	defaulted := u.DeepCopy()
	defaults(defaulted.Object)
	if equality.Semantic.DeepEqual(defaulted.Object, u.Object) {
		return written
	}
	if err := custom.Update(action.GetResource(), defaulted, action.GetNamespace()); err != nil {
		return written
	}
	return defaulted
}

// unstructuredFromTyped returns a typed object, or list, the typed tracker
// answered as unstructured, with apiVersion and kind set on it and on each
// item: the dynamic client converts what a reactor returns with its own
// scheme, which need not know the built-in Go types.
func unstructuredFromTyped(obj runtime.Object) (runtime.Object, error) {
	if _, ok := obj.(runtime.Unstructured); ok {
		return obj, nil
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: m}
	kinds, _, err := clientgoscheme.Scheme.ObjectKinds(obj)
	if err != nil || len(kinds) == 0 {
		return u, nil
	}
	u.SetGroupVersionKind(kinds[0])
	if items, ok := m["items"].([]any); ok {
		itemKind := kinds[0].GroupVersion().WithKind(strings.TrimSuffix(kinds[0].Kind, "List"))
		for _, it := range items {
			if im, ok := it.(map[string]any); ok {
				im["apiVersion"], im["kind"] = itemKind.GroupVersion().String(), itemKind.Kind
			}
		}
	}
	return u, nil
}

// withTypedObject returns action with an unstructured object it writes
// converted to gvk's Go type.
func withTypedObject(action k8stesting.Action, gvk schema.GroupVersionKind) (k8stesting.Action, error) {
	switch a := action.(type) {
	case k8stesting.CreateActionImpl:
		obj, err := typedFrom(a.Object, gvk)
		a.Object = obj
		return a, err
	case k8stesting.UpdateActionImpl:
		obj, err := typedFrom(a.Object, gvk)
		a.Object = obj
		return a, err
	}
	return action, nil
}

func typedFrom(obj runtime.Object, gvk schema.GroupVersionKind) (runtime.Object, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}
	typed, err := clientgoscheme.Scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, typed); err != nil {
		return nil, fmt.Errorf("converting %s %s: %w", gvk.Kind, u.GetName(), err)
	}
	return typed, nil
}

// kindOfWrite returns the kind an action writes, from the object it carries
// or, for a patch, the apiVersion and kind in its body; empty when it says
// none.
func kindOfWrite(action k8stesting.Action) schema.GroupVersionKind {
	switch a := action.(type) {
	case k8stesting.CreateAction:
		return a.GetObject().GetObjectKind().GroupVersionKind()
	case k8stesting.UpdateAction:
		return a.GetObject().GetObjectKind().GroupVersionKind()
	case k8stesting.PatchAction:
		var head struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
		}
		if json.Unmarshal(a.GetPatch(), &head) == nil && head.Kind != "" {
			return schema.FromAPIVersionAndKind(head.APIVersion, head.Kind)
		}
	}
	return schema.GroupVersionKind{}
}

// registerKind makes the scheme know gvk, as unstructured, when nothing has
// registered it: the field-managed tracker resolves every write's kind
// through the scheme, and a child custom resource from another operator's
// CRD is in no Blueprint's scheme.
func (f *Kubeclient) registerKind(gvk schema.GroupVersionKind) {
	if gvk.Kind == "" || f.scheme == nil || f.scheme.Recognizes(gvk) {
		return
	}
	f.scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	if list := gvk.GroupVersion().WithKind(gvk.Kind + "List"); !f.scheme.Recognizes(list) {
		f.scheme.AddKnownTypeWithName(list, &unstructured.UnstructuredList{})
	}
}

// resourceOf returns obj's resource and kind: from its TypeMeta, or its Go
// type through the scheme; the resource is the built-in one when obj is
// built in.
func (f *Kubeclient) resourceOf(obj runtime.Object) (schema.GroupVersionResource, schema.GroupVersionKind, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		for _, s := range []*runtime.Scheme{f.scheme, clientgoscheme.Scheme} {
			if s == nil {
				continue
			}
			if kinds, _, err := s.ObjectKinds(obj); err == nil && len(kinds) > 0 {
				gvk = kinds[0]
				break
			}
		}
	}
	if gvk.Empty() {
		return schema.GroupVersionResource{}, gvk, fmt.Errorf("no kind known for %T", obj)
	}
	_, byGVK := builtins()
	if gvr, ok := byGVK[gvk]; ok {
		return gvr, gvk, nil
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr, gvk, nil
}

// foldStoredSecret folds a written Secret's stringData into data in the
// typed tracker, as the API server does on every write, and returns the
// stored Secret; any other object is returned as it is.
func foldStoredSecret(typed k8stesting.ObjectTracker, action k8stesting.Action, written runtime.Object) runtime.Object {
	sec, ok := written.(*corev1.Secret)
	if !ok {
		return written
	}
	sec = sec.DeepCopy()
	if !foldStringData(sec) {
		return written
	}
	if err := typed.Update(action.GetResource(), sec, sec.GetNamespace()); err != nil {
		return written
	}
	return sec
}

// withoutNamespace returns action with metadata.namespace cleared from the
// object it writes, as the API server stores a cluster-scoped object: a
// client builds one in its owner's namespace and sends it cluster-wide.
func withoutNamespace(action k8stesting.Action) k8stesting.Action {
	switch a := action.(type) {
	case k8stesting.CreateActionImpl:
		if u, ok := a.Object.(*unstructured.Unstructured); ok && u.GetNamespace() != "" {
			cp := u.DeepCopy()
			cp.SetNamespace("")
			a.Object = cp
		}
		return a
	case k8stesting.UpdateActionImpl:
		if u, ok := a.Object.(*unstructured.Unstructured); ok && u.GetNamespace() != "" {
			cp := u.DeepCopy()
			cp.SetNamespace("")
			a.Object = cp
		}
		return a
	case k8stesting.PatchActionImpl:
		var body map[string]interface{}
		if json.Unmarshal(a.Patch, &body) != nil {
			return a
		}
		if md, ok := body["metadata"].(map[string]interface{}); ok {
			if _, has := md["namespace"]; has {
				delete(md, "namespace")
				if raw, err := json.Marshal(body); err == nil {
					a.Patch = raw
				}
			}
		}
		return a
	}
	return action
}

// foldSecrets serves a typed-client action on Secrets from the typed tracker
// and folds what a write left there: the typed clientset writes straight to
// its tracker, not through storeReaction, and Inrun writes Secrets with it.
func foldSecrets(typed k8stesting.ObjectTracker) k8stesting.ReactionFunc {
	react := k8stesting.ObjectReaction(typed)
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		handled, obj, err := react(action)
		if err != nil || obj == nil || !isWriteAction(action) {
			return handled, obj, err
		}
		return handled, foldStoredSecret(typed, action, obj), nil
	}
}
