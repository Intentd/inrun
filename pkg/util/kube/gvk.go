package kube

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/logger"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersionKind is a consistent string key for GVK-keyed maps and logs.
type GroupVersionKind string

// SetGroupVersionKind formats a GVK as canonical "group/version, Kind=Kind".
func SetGroupVersionKind(group, version, kind string) GroupVersionKind {
	return GroupVersionKind(fmt.Sprintf("%s/%s, Kind=%s", group, version, kind))
}

// SetGroupVersionKindObj formats a schema GVK as a string.
func SetGroupVersionKindObj(gvk schema.GroupVersionKind) string {
	return SetGroupVersionKind(gvk.Group, gvk.Version, gvk.Kind).String()
}

// String returns the underlying string.
func (g GroupVersionKind) String() string {
	return string(g)
}

// GvkFromObject resolves a GVK from the scheme (cached objects lack TypeMeta).
func GvkFromObject(obj interface{}, scheme *runtime.Scheme) (*schema.GroupVersionKind, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		logger.Error().Str("type", fmt.Sprintf("%T", obj)).Msg("object is not a runtime.Object — event dropped")
		return nil, fmt.Errorf("object is not a runtime.Object: %T", obj)
	}

	// Cached objects have TypeMeta stripped, so resolve via scheme.
	gvks, _, err := scheme.ObjectKinds(runtimeObj)
	if err != nil || len(gvks) == 0 {
		logger.Error().Err(err).Str("type", fmt.Sprintf("%T", obj)).Msg("failed to resolve GVK — event dropped")
		return nil, fmt.Errorf("failed to resolve GVK for %T — event dropped", obj)
	}

	return &gvks[0], nil
}

// GVKFromFields builds a GVK from apiVersion and kind strings.
func GVKFromFields(apiVersion, kind string) (schema.GroupVersionKind, error) {
	if apiVersion == "" || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion and kind are required")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid apiVersion %q: %w", apiVersion, err)
	}
	return gv.WithKind(kind), nil
}

// GVRFromFields builds a GVR from apiVersion and kind strings, with plural as
// the resource when given and PluralForKind(kind) otherwise. Use it where no
// cluster can be asked; with a cluster, kubeclient.GVRForGVK is exact.
func GVRFromFields(apiVersion, kind, plural string) (schema.GroupVersionResource, error) {
	gvk, err := GVKFromFields(apiVersion, kind)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	if plural == "" {
		plural = PluralForKind(kind)
	}
	return gvk.GroupVersion().WithResource(strings.ToLower(plural)), nil
}

// PluralForKind returns the resource name Kubernetes would guess for kind,
// lowercase and pluralised the way the API does (Policy → policies, Ingress →
// ingresses). A CRD may declare a different plural: use it when it is known.
func PluralForKind(kind string) string {
	gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: kind})
	return gvr.Resource
}

// GVRKey is the map key for a resource: group/version/resource (or version/resource in core).
func GVRKey(group, version, resource string) string {
	return groupVersionKey(group, version, resource)
}

// GVKKey is the map key for a kind: group/version/kind (or version/kind in core).
func GVKKey(group, version, kind string) string {
	return groupVersionKey(group, version, kind)
}

// groupVersionKey builds a group/version/name key, omitting the group when core.
func groupVersionKey(group, version, name string) string {
	if group == "" {
		return version + "/" + name
	}
	return group + "/" + version + "/" + name
}
