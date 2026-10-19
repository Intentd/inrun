package kube

import (
	"fmt"

	"github.com/intentd/inrun/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersionKind is a consistent string key for GVK-keyed maps and logs.
type GroupVersionKind string

// SetGroupVersionKind formats a GVK as the canonical string used internally.
// Mirrors runtime.Object.GetObjectKind().GroupVersionKind().String() output.
func SetGroupVersionKind(group, version, kind string) GroupVersionKind {
	return GroupVersionKind(fmt.Sprintf("%s/%s, Kind=%s", group, version, kind))
}

func SetGroupVersionKindObj(gvk schema.GroupVersionKind) string {
	return SetGroupVersionKind(gvk.Group, gvk.Version, gvk.Kind).String()
}

func (g GroupVersionKind) String() string {
	return string(g)
}

func GvkFromObject(obj interface{}, scheme *runtime.Scheme) (*schema.GroupVersionKind, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		logger.Error().Str("type", fmt.Sprintf("%T", obj)).Msg("object is not a runtime.Object — event dropped")
		return nil, fmt.Errorf("object is not a runtime.Object: %T", obj)
	}

	// Resolve GVK from scheme — cached objects have TypeMeta stripped,
	// GetObjectKind().GroupVersionKind() returns empty.
	gvks, _, err := scheme.ObjectKinds(runtimeObj)
	if err != nil || len(gvks) == 0 {
		logger.Error().Err(err).Str("type", fmt.Sprintf("%T", obj)).Msg("failed to resolve GVK — event dropped")
		return nil, fmt.Errorf("failed to resolve GVK for %T — event dropped", obj)
	}

	return &gvks[0], nil
}

func GVKFromFields(apiVersion, kind string) (schema.GroupVersionKind, error) {
	if apiVersion == "" || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion and kind are required")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return gv.WithKind(kind), nil
}

// GVRKey is the map key for a resource: group/version/resource, or
// version/resource in the core group. The gateway's webhooks, its Blueprint
// handler and the Blueprint's admission registry all key by it.
func GVRKey(group, version, resource string) string {
	return groupVersionKey(group, version, resource)
}

// GVKKey is the map key for a kind: group/version/kind, or version/kind in
// the core group.
func GVKKey(group, version, kind string) string {
	return groupVersionKey(group, version, kind)
}

func groupVersionKey(group, version, name string) string {
	if group == "" {
		return version + "/" + name
	}
	return group + "/" + version + "/" + name
}
