package apply

import (
	"fmt"
	"github.com/intentd/inrun/pkg/labels"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/kubeclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// metadata carries optional labels and annotations to apply to a built object.
// Empty maps are ignored, so callers can pass a zero-value metadata{} freely.
type metadata struct {
	Labels      map[string]string
	Annotations map[string]string
}

// enrichedObjFor builds an Unstructured for gvk, taking name and namespace
// from obj. Labels and annotations are set only when non-empty.
func enrichedObjFor(gvk schema.GroupVersionKind, obj domain.Object, m metadata) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(obj.GetName())
	u.SetNamespace(obj.GetNamespace())
	if len(m.Labels) > 0 {
		u.SetLabels(m.Labels)
	}
	if len(m.Annotations) > 0 {
		u.SetAnnotations(m.Annotations)
	}
	return u
}

// managesMetadata reports whether FieldManager owns any field on the object
// itself, so a Result that drops all its keys still clears them.
func managesMetadata(obj domain.Object) bool {
	for _, mf := range obj.GetManagedFields() {
		if mf.Manager == FieldManager && mf.Operation == metav1.ManagedFieldsOperationApply && mf.Subresource == "" {
			return true
		}
	}
	return false
}

// resolveTargets parses every expanded resource, resolves its REST mapping, and
// verifies it's declared in managed. It returns the parsed targets only if all
// of them pass — nothing is applied until the whole set is validated.
func resolveTargets(
	kube kubeclient.Interface,
	expanded []map[string]interface{},
	managed []domain.ManagedResource,
) ([]resource, error) {
	allowed := make(map[string]struct{}, len(managed))
	for _, mr := range managed {
		if gvr, ok := children.ResolveGVR(mr); ok {
			allowed[allowKey(gvr.Group, gvr.Resource)] = struct{}{}
		}
		allowed[allowKey(mr.Group, mr.Plural)] = struct{}{}
	}

	// Parse and check everything before applying anything.
	targets := make([]resource, 0, len(expanded))
	for i, m := range expanded {
		rr, err := parse(m)
		if err != nil {
			return nil, fmt.Errorf("resources[%d]: %w", i, err)
		}
		mapping, err := kubeclient.GVRFor(kube, rr.obj)
		if err != nil {
			return nil, fmt.Errorf("resources[%d] %q (%s): no REST mapping — is the CRD installed? %w",
				i, rr.obj.GetName(), rr.obj.GroupVersionKind(), err)
		}
		if !isAllowedResource(allowed, mapping.Resource) {
			return nil, fmt.Errorf("resources[%d] %q (%s/%s) is not declared in managedResources — add it to the Catalog before the reconciler can create it",
				i, rr.obj.GetName(), mapping.Resource.Group, mapping.Resource.Resource)
		}
		targets = append(targets, rr)
	}
	return targets, nil
}

// isAllowedResource reports whether gvr is declared in allowed.
// Keys are "group/resource", with the core group written as "" (e.g. "/services").
func isAllowedResource(allowed map[string]struct{}, gvr schema.GroupVersionResource) bool {
	_, ok := allowed[allowKey(gvr.Group, gvr.Resource)]
	return ok
}

// allowKey builds the lookup key for a resource type. The core group is the
// empty string, so a Service produces "/services".
func allowKey(group, resource string) string {
	return group + "/" + resource
}

// managedSince is when Inrun started managing owner: its managed-since
// annotation, else its creation time. Children carry the same value, so it
// is stable across applies.
func managedSince(owner domain.Object) string {
	if v := owner.GetAnnotations()[labels.AnnotationManagedSince]; v != "" {
		return v
	}
	if ts := owner.GetCreationTimestamp(); !ts.IsZero() {
		return ts.UTC().Format(time.RFC3339)
	}
	return ""
}
