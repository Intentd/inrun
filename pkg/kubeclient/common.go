package kubeclient

import (
	"context"
	"encoding/json"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	sigs "sigs.k8s.io/controller-runtime/pkg/client"
)

// GVKFor resolves the object's GroupVersionKind from the client's scheme.
func GVKFor(c Interface, obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := c.Scheme().ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("unknown type %T: %w", obj, err)
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("no GVK registered for %T", obj)
	}
	return gvks[0], nil
}

// GVRFor resolves the REST mapping used to address the object's resource.
func GVRFor(c Interface, obj runtime.Object) (*apimeta.RESTMapping, error) {
	gvk, err := GVKFor(c, obj)
	if err != nil {
		return nil, err
	}
	return RestMappingForGVK(c, gvk)
}

// ResourceFor returns the dynamic resource interface at the object's scope.
func ResourceFor(c Interface, obj sigs.Object) (dynamic.ResourceInterface, error) {
	mapping, err := GVRFor(c, obj)
	if err != nil {
		return nil, err
	}

	resource := c.DynamicClient().Resource(mapping.Resource)

	if !isClusterScoped(mapping) {
		return resource.Namespace(obj.GetNamespace()), nil
	}
	return resource, nil
}

// PatchUnstructured dispatches a patch to the correct scope — namespaced or
// cluster-scoped — based on the object's own namespace field.
func PatchUnstructured(
	ctx context.Context,
	resource dynamic.NamespaceableResourceInterface,
	obj *unstructured.Unstructured,
	pt k8stypes.PatchType,
	data []byte,
	opts metav1.PatchOptions,
) (*unstructured.Unstructured, error) {
	if ns := obj.GetNamespace(); ns != "" {
		return resource.Namespace(ns).Patch(ctx, obj.GetName(), pt, data, opts)
	}
	return resource.Patch(ctx, obj.GetName(), pt, data, opts)
}

// ListAllForNamespaces lists all objects across the given namespaces. When
// clusterScoped is true or namespaces is empty, a single unscoped list is
// performed instead. Pagination is handled internally.
func ListAllForNamespaces(ctx context.Context, resource dynamic.NamespaceableResourceInterface, clusterScoped bool, namespaces []string, opts metav1.ListOptions) ([]unstructured.Unstructured, error) {
	if clusterScoped || len(namespaces) == 0 {
		return ListAll(ctx, resource, opts)
	}
	var all []unstructured.Unstructured
	for _, ns := range namespaces {
		items, err := ListAllInNamespace(ctx, resource, ns, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}
	return all, nil
}

// ListAllInNamespace lists all objects for a namespaced resource, following
// continuation tokens so the caller gets a single flat slice regardless of
// how many pages the API server returns.
func ListAllInNamespace(ctx context.Context, resource dynamic.NamespaceableResourceInterface, ns string, opts metav1.ListOptions) ([]unstructured.Unstructured, error) {
	return listAll(ctx, resource.Namespace(ns), opts)
}

// ListAll lists all objects for a cluster-scoped resource (or an unscoped
// list across all namespaces), following continuation tokens.
func ListAll(ctx context.Context, resource dynamic.NamespaceableResourceInterface, opts metav1.ListOptions) ([]unstructured.Unstructured, error) {
	return listAll(ctx, resource, opts)
}

func listAll(ctx context.Context, lister dynamic.ResourceInterface, opts metav1.ListOptions) ([]unstructured.Unstructured, error) {
	var items []unstructured.Unstructured
	for {
		page, err := lister.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		opts.Continue = page.GetContinue()
		if opts.Continue == "" {
			return items, nil
		}
	}
}

// isClusterScoped reports whether a REST mapping is for a cluster-scoped resource.
func isClusterScoped(m *apimeta.RESTMapping) bool {
	return m.Scope.Name() == apimeta.RESTScopeNameRoot
}

// RestMappingForGVK resolves the REST mapping for a GVK.
func RestMappingForGVK(c Interface, gvk schema.GroupVersionKind) (*apimeta.RESTMapping, error) {
	mapping, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("no REST mapping for %s: %w. Check that the CRD is installed", gvk, err)
	}
	return mapping, nil
}

// DynamicResourceFor resolves a namespaceable dynamic resource interface for a
// GVK, plus a bool reporting whether the resource is cluster-scoped. The caller
// controls namespace selection; for a scoped resource use
// resource.Namespace(ns), for a cluster-scoped one use resource directly.
func DynamicResourceFor(c Interface, gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, bool, error) {
	mapping, err := RestMappingForGVK(c, gvk)
	if err != nil {
		return nil, false, err
	}
	resource := c.DynamicClient().Resource(mapping.Resource)
	return resource, isClusterScoped(mapping), nil
}

// ResourceForGVK resolves a dynamic resource interface for a GVK and
// applies the namespace when the mapped resource is namespaced.
func ResourceForGVK(c Interface, gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	resource, clusterScoped, err := DynamicResourceFor(c, gvk)
	if err != nil {
		return nil, err
	}
	if !clusterScoped {
		return resource.Namespace(namespace), nil
	}
	return resource, nil
}

// ToUnstructured converts a controller-runtime object to an unstructured object.
func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

// GroupVersionKindFor resolves the object's GVK from the client's scheme.
func GroupVersionKindFor(c Interface, obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := c.Scheme().ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf(
			"kubeclient: no GVK registered for %T",
			obj,
		)
	}
	return gvks[0], nil
}

// GVRForGVK resolves a GroupVersionKind to its GroupVersionResource via the
// client's RESTMapper. Returns an error when the GVK cannot be mapped
func GVRForGVK(c Interface, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("kubeclient: no REST mapping for %s: %w", gvk, err)
	}
	return mapping.Resource, nil
}

// GVKForGVR resolves a GroupVersionResource to its GroupVersionKind via the
// client's RESTMapper. Returns an error when the GVR cannot be mapped.
func GVKForGVR(c Interface, gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	gvk, err := c.RESTMapper().KindFor(gvr)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("kubeclient: no kind for %s: %w", gvr, err)
	}
	return gvk, nil
}

// IsObjectNamespaced reports whether the object's resource is namespace-scoped.
func IsObjectNamespaced(c Interface, obj runtime.Object) (bool, error) {
	if c == nil {
		return false, nil
	}
	mapping, err := GVRFor(c, obj)
	if err != nil {
		return false, err
	}
	return !isClusterScoped(mapping), nil
}

// RunApply executes a server-side apply against the target resource,
// optionally addressing a subresource such as "status".
func RunApply(
	ctx context.Context,
	c Interface,
	cfg runtime.ApplyConfiguration,
	opts metav1.ApplyOptions,
	subresource ...string,
) error {
	if cfg == nil {
		return fmt.Errorf("apply configuration must not be nil")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal apply configuration: %w", err)
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, &u.Object); err != nil {
		return fmt.Errorf("decode apply configuration: %w", err)
	}

	gvk := u.GroupVersionKind()
	if gvk.Empty() {
		return fmt.Errorf("apply configuration %T has no apiVersion/kind", cfg)
	}

	name := u.GetName()
	ns := u.GetNamespace()

	if name == "" {
		return fmt.Errorf("apply configuration %T has no metadata.name", cfg)
	}

	resource, err := ResourceForGVK(c, gvk, ns)
	if err != nil {
		return err
	}

	result, err := resource.Apply(
		ctx,
		name,
		u,
		opts,
		subresource...,
	)
	if err != nil {
		return err
	}

	resultData, err := json.Marshal(result.Object)
	if err != nil {
		return fmt.Errorf("marshal applied object: %w", err)
	}

	return json.Unmarshal(resultData, cfg)
}
