// Package apply performs the writes a reconciler's Result asks for: its
// resources, and the labels and annotations on the CR. The coordinator and
// inrun simulate both call it, so every reconciler's writes take one path.
package apply

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
	resapply "github.com/intentd/kubewright/core/apply"
	kwtypes "github.com/intentd/kubewright/core/types"
)

// FieldManager is the server-side apply field manager for resources a
// reconciler returns.
const FieldManager = "inrun-reconciler"

// resource pairs a parsed object with its optional per-resource forceConflict
// override, taken from the raw map before SSA.
type resource struct {
	obj           *unstructured.Unstructured
	forceConflict *bool
}

// Result applies a Result's resources, prunes children it no longer returns
// when prune is enabled, then applies its labels and annotations on owner.
// It returns the children it pruned.
func Result(
	ctx context.Context,
	kube kubeclient.Interface,
	owner domain.Object,
	managed []domain.ManagedResource,
	prune *types.PruneConfig,
	r domain.Result,
) ([]Pruned, error) {
	applied, err := apply(ctx, kube, owner, managed, r.Resources)
	if err != nil {
		return nil, err
	}
	pruned, err := Prune(ctx, kube.DynamicClient(), owner, managed, prune, applied)
	if err != nil {
		return pruned, err
	}
	return pruned, Metadata(ctx, kube, owner, r.Labels, r.Annotations)
}

// Resources expands, checks and applies the resources a Result returns.
//
// Every resource is checked against managed before any is applied. Resources
// in the owner's namespace get an owner reference so Kubernetes garbage
// collection removes them with the CR.
func Resources(
	ctx context.Context,
	kube kubeclient.Interface,
	owner domain.Object,
	managed []domain.ManagedResource,
	raw []map[string]interface{},
) error {
	_, err := apply(ctx, kube, owner, managed, raw)
	return err
}

// apply is Resources, returning the objects it applied for Prune.
func apply(
	ctx context.Context,
	kube kubeclient.Interface,
	owner domain.Object,
	managed []domain.ManagedResource,
	raw []map[string]interface{},
) ([]*unstructured.Unstructured, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	expanded, err := Expand(raw, owner)
	if err != nil {
		return nil, err
	}

	targets, targetErr := resolveTargets(kube, expanded, managed)
	if targetErr != nil {
		return nil, targetErr
	}

	blueprintName := owner.GetAnnotations()[labels.AnnotationManagedBy]
	since := managedSince(owner)

	applied := make([]*unstructured.Unstructured, 0, len(targets))
	for i, rr := range targets {
		if err := applyOne(ctx, kube, rr, owner, blueprintName, since); err != nil {
			return nil, fmt.Errorf("resources[%d] %q: apply failed: %w", i, rr.obj.GetName(), err)
		}
		applied = append(applied, rr.obj)
	}
	return applied, nil
}

// Expand turns intent-form entries ({"type", "fields"}) into full Kubernetes
// objects. Full-form entries (with apiVersion) pass through unchanged.
func Expand(raw []map[string]interface{}, owner domain.Object) ([]map[string]interface{}, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for i, entry := range raw {
		if _, hasAPI := entry["apiVersion"]; hasAPI {
			out = append(out, entry)
			continue
		}
		typVal, hasType := entry["type"]
		fieldsVal, hasFields := entry["fields"]
		if !hasType || !hasFields {
			return nil, fmt.Errorf("resources[%d]: expected 'apiVersion' (full form) or 'type'+'fields' (intent form)", i)
		}
		typ, ok := typVal.(string)
		if !ok {
			return nil, fmt.Errorf("resources[%d]: 'type' must be a string", i)
		}
		fields, ok := fieldsVal.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("resources[%d]: 'fields' must be an object", i)
		}
		built, err := intent.BuildResource(typ, fields, owner)
		if err != nil {
			return nil, fmt.Errorf("resources[%d]: %w", i, err)
		}
		out = append(out, built)
	}
	return out, nil
}

// parse takes the optional forceConflict field out of the raw map, so
// Kubernetes never sees it, and unmarshals the rest.
func parse(raw map[string]interface{}) (resource, error) {
	var fc *bool
	if v, ok := raw["forceConflict"]; ok {
		if b, ok := v.(bool); ok {
			fc = &b
		}
		delete(raw, "forceConflict")
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return resource{}, fmt.Errorf("marshal: %w", err)
	}
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, &u.Object); err != nil {
		return resource{}, fmt.Errorf("unmarshal: %w", err)
	}
	if u.GetName() == "" {
		return resource{}, fmt.Errorf("resource has no metadata.name")
	}
	if u.GroupVersionKind().Empty() {
		return resource{}, fmt.Errorf("resource %q has no apiVersion/kind", u.GetName())
	}
	return resource{obj: u, forceConflict: fc}, nil
}

func applyOne(
	ctx context.Context,
	kube kubeclient.Interface,
	rr resource,
	owner domain.Object,
	blueprintName string,
	since string,
) error {
	u := rr.obj

	// Management labels and annotations: the same invariants as declarative
	// resources. managed-since comes from the owner, so every apply sends the
	// same value and an unchanged child is a no-op, not an update event.
	if since != "" {
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		if ann[labels.AnnotationManagedSince] == "" {
			ann[labels.AnnotationManagedSince] = since
			u.SetAnnotations(ann)
		}
	}
	// The same ownership labels as declarative children: the console finds a
	// CR's children by them, and the surface sweep uses them on target changes.
	// The owner UID label lets Prune find exactly this CR's children.
	u.SetLabels(labels.StampInrunLabels(u.GetLabels(), owner))
	labels.NewManager(labels.Config{}).EnsureManagedAnnotations(u, blueprintName)

	// Owner reference, wherever Kubernetes honours one: a namespaced owner
	// owns only children in its own namespace (kwtypes.CanOwn). The rest are
	// found by the owner-UID label, for prune and the finalize sweep.
	if refs := kwtypes.OwnerReferences(owner, u.GetNamespace()); refs != nil {
		u.SetOwnerReferences(refs)
	}

	_, err := resapply.Object(ctx, kube.DynamicClient(), kube.RESTMapper(), u, kwtypes.Options{
		FieldManager: FieldManager,
		Force:        kubeclient.ApplyOptions(kube, rr.forceConflict).Force,
	})
	return err
}
