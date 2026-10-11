package http

import (
	"strings"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/children"
	"github.com/inrundev/inrun/pkg/types"
	"github.com/inrundev/inrun/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// childObjects are the children of one type, keyed by name.
type childObjects map[string]map[string]interface{}

// childrenMap is the request's children, keyed by type ("deployment",
// "service"), then by name.
type childrenMap map[string]childObjects

// kindToTypeName converts a Kubernetes Kind to the lowercase intent-form type name.
// For custom types — the kind is used as-is without a registry check.
func kindToTypeName(kind string) (string, bool) {
	if kind == "" {
		return "", false
	}
	if !children.IsBuiltIn(kind) {
		return strings.ToLower(kind), true
	}
	res := children.LookupBuiltIn(kind)
	if !res.Found() {
		return "", false
	}
	return strings.ToLower(res.Kind()), true
}

// listChildren reads real cluster state for each managed resource type owned by the
// CR from the informer store, building a children map for the outbound payload.
// Returns nil when injection is disabled, no informer store is available, or no owned resources are found.
func (r *Reconciler) listChildren(owner domain.Object) childrenMap {
	if !r.childrenEnabled() {
		return nil
	}
	if owner == nil {
		return nil
	}
	storeFor := r.kube.GetStoreFor()
	if storeFor == nil {
		return nil
	}

	ownerUID := owner.GetUID()
	cm := childrenMap{}

	for _, mr := range r.managedResources {
		gvk, ok := children.ResolveGVK(mr)
		if !ok {
			continue
		}

		kind := gvk.Kind
		if kind == "" {
			continue
		}
		store := storeFor(gvk)
		if store == nil {
			continue
		}
		typeName, ok := kindToTypeName(kind)
		if !ok {
			continue
		}
		for _, item := range store.List() {
			obj, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !isOwnedBy(obj, ownerUID) {
				continue
			}
			name := obj.GetName()
			if name == "" {
				continue
			}
			if cm[typeName] == nil {
				cm[typeName] = childObjects{}
			}
			cm[typeName][name] = obj.Object
		}
	}

	if len(cm) == 0 {
		return nil
	}

	return r.applyChildrenConfig(cm)
}

// isOwnedBy reports whether obj has an owner reference pointing to ownerUID.
func isOwnedBy(obj metav1.Object, ownerUID k8stypes.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}
	return false
}

// applyChildrenConfig filters by resources allow-list and strips excluded paths.
// Per-resource exclude takes precedence over the root exclude for each type.
func (r *Reconciler) applyChildrenConfig(cm childrenMap) childrenMap {
	cfg := r.childrenConfig()

	// Filter to allowed resource types when resources map is declared.
	if cfg.HasResources() {
		filtered := childrenMap{}
		for typeName, objs := range cm {
			if _, ok := cfg.Resources[typeName]; ok {
				filtered[typeName] = objs
			}
		}
		cm = filtered
	}

	// Strip paths: per-resource exclude overrides root exclude for that type.
	for typeName, objs := range cm {
		var paths []string
		if res, ok := cfg.Resources[typeName]; ok && res != nil {
			paths = res.Exclude
		}
		if paths == nil {
			paths = cfg.Exclude
		}
		if len(paths) == 0 {
			continue
		}
		stripped := make(childObjects, len(objs))
		for name, obj := range objs {
			cp := make(map[string]interface{}, len(obj))
			for k, v := range obj {
				cp[k] = v
			}
			for _, path := range paths {
				utils.DeleteNestedPath(cp, path)
			}
			stripped[name] = cp
		}
		cm[typeName] = stripped
	}

	if len(cm) == 0 {
		return nil
	}
	return cm
}

// childrenEnabled reports whether children injection is active for this reconciler.
func (r *Reconciler) childrenEnabled() bool {
	return r.decl.Request.ChildrenEnabled()
}

// childrenConfig returns the effective children config; nil when children are
// disabled, so check childrenEnabled first.
func (r *Reconciler) childrenConfig() *types.RemoteRequestChildrenConfig {
	return r.decl.Request.EffectiveChildren()
}
