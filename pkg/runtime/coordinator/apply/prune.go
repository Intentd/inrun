package apply

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
)

// Pruned names a child deleted because the Result no longer returned it.
type Pruned struct {
	Kind      string
	Namespace string
	Name      string
}

// Prune deletes the children Inrun applied for owner on an earlier reconcile
// that applied (this reconcile's resources) no longer contains.
//
// A child is a candidate only if it carries owner's UID label, has
// inrun-reconciler among its field managers, is of a managed kind not listed
// in prune.keep, and is not deletion-protected. Nothing is pruned when prune
// is off, or when applied is empty and prune.allowEmpty is false. Callers run
// it only after every apply succeeded.
func Prune(
	ctx context.Context,
	dyn dynamic.Interface,
	owner domain.Object,
	managed []domain.ManagedResource,
	prune *types.PruneConfig,
	applied []*unstructured.Unstructured,
) ([]Pruned, error) {
	if !prune.IsEnabled() || owner.GetUID() == "" {
		return nil, nil
	}
	if len(applied) == 0 && !prune.AllowEmpty {
		return nil, nil
	}

	want := make(map[string]struct{}, len(applied))
	for _, u := range applied {
		want[childKey(u.GroupVersionKind().Group, u.GetKind(), u.GetNamespace(), u.GetName())] = struct{}{}
	}

	opts := metav1.ListOptions{LabelSelector: labels.OwnerUID + "=" + string(owner.GetUID())}
	background := metav1.DeletePropagationBackground

	var pruned []Pruned
	for _, gvr := range pruneGVRs(managed, prune.Keep) {
		// Namespace("") lists across namespaces for namespaced kinds and is
		// the plain list for cluster-scoped ones.
		list, err := dyn.Resource(gvr).Namespace("").List(ctx, opts)
		if err != nil {
			return pruned, fmt.Errorf("prune: list %s: %w", gvr.Resource, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			if !appliedByInrun(item) || item.GetLabels()[labels.DeletionProtectionLabel] == "true" {
				continue
			}
			key := childKey(item.GroupVersionKind().Group, item.GetKind(), item.GetNamespace(), item.GetName())
			if _, ok := want[key]; ok {
				continue
			}
			err := dyn.Resource(gvr).Namespace(item.GetNamespace()).Delete(ctx, item.GetName(),
				metav1.DeleteOptions{PropagationPolicy: &background})
			if err != nil && !apierrors.IsNotFound(err) {
				return pruned, fmt.Errorf("prune: delete %s %s/%s: %w", item.GetKind(), item.GetNamespace(), item.GetName(), err)
			}
			pruned = append(pruned, Pruned{Kind: item.GetKind(), Namespace: item.GetNamespace(), Name: item.GetName()})
		}
	}
	return pruned, nil
}

// PruneChildren deletes every child of owner, whoever wrote it (a Result,
// templates, hooks): every object of the kinds in gvrs carrying owner's UID
// label, in any namespace and cluster-scoped ones included, except kinds in
// keep and deletion-protected objects. The runtime calls it once a finalizing
// reconcile is done: owner references cover only same-namespace children, so
// the rest would outlive the CR.
func PruneChildren(
	ctx context.Context,
	dyn dynamic.Interface,
	owner domain.Object,
	gvrs []schema.GroupVersionResource,
	keep []string,
) ([]Pruned, error) {
	if owner.GetUID() == "" {
		return nil, nil
	}
	return PruneOwned(ctx, dyn, labels.OwnerUID+"="+string(owner.GetUID()), gvrs, keep)
}

// PruneOwned deletes every object of the kinds in gvrs that matches the
// label selector, in any namespace and cluster-scoped ones included, except
// kinds in keep and deletion-protected objects. The selector must name an
// owner (labels.OwnerUID); an empty one deletes nothing.
func PruneOwned(
	ctx context.Context,
	dyn dynamic.Interface,
	selector string,
	gvrs []schema.GroupVersionResource,
	keep []string,
) ([]Pruned, error) {
	if !strings.Contains(selector, labels.OwnerUID+"=") {
		return nil, nil
	}
	kept := make(map[string]bool, len(keep))
	for _, k := range keep {
		kept[children.KindName(k)] = true
		kept[strings.ToLower(k)] = true
	}
	background := metav1.DeletePropagationBackground
	var deleted []Pruned
	for _, gvr := range gvrs {
		if kept[strings.ToLower(gvr.Resource)] {
			continue
		}
		list, err := dyn.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // the kind isn't served here
			}
			return deleted, fmt.Errorf("list %s: %w", gvr.Resource, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			if kept[children.KindName(item.GetKind())] || item.GetLabels()[labels.DeletionProtectionLabel] == "true" {
				continue
			}
			err := dyn.Resource(gvr).Namespace(item.GetNamespace()).Delete(ctx, item.GetName(),
				metav1.DeleteOptions{PropagationPolicy: &background})
			if err != nil && !apierrors.IsNotFound(err) {
				return deleted, fmt.Errorf("delete %s %s/%s: %w", item.GetKind(), item.GetNamespace(), item.GetName(), err)
			}
			deleted = append(deleted, Pruned{Kind: item.GetKind(), Namespace: item.GetNamespace(), Name: item.GetName()})
		}
	}
	return deleted, nil
}

// ManagedGVRs resolves managed resources to the resources to list, without
// duplicates.
func ManagedGVRs(managed []domain.ManagedResource) []schema.GroupVersionResource {
	return pruneGVRs(managed, nil)
}

// pruneGVRs resolves the managed kinds to list, without duplicates and
// without the kinds in keep.
func pruneGVRs(managed []domain.ManagedResource, keep []string) []schema.GroupVersionResource {
	kept := make(map[string]struct{}, len(keep))
	for _, k := range keep {
		kept[children.KindName(k)] = struct{}{}
	}
	seen := make(map[schema.GroupVersionResource]struct{}, len(managed))
	out := make([]schema.GroupVersionResource, 0, len(managed))
	for _, mr := range managed {
		if _, ok := kept[children.KindName(mr.Kind)]; ok && mr.Kind != "" {
			continue
		}
		if _, ok := kept[strings.ToLower(mr.Plural)]; ok && mr.Plural != "" {
			continue
		}
		gvr, ok := children.ResolveGVR(mr)
		if !ok {
			continue
		}
		if _, dup := seen[gvr]; dup {
			continue
		}
		seen[gvr] = struct{}{}
		out = append(out, gvr)
	}
	return out
}

// appliedByInrun reports whether a Result apply wrote obj: FieldManager
// appears among its managed fields.
func appliedByInrun(obj *unstructured.Unstructured) bool {
	for _, mf := range obj.GetManagedFields() {
		if mf.Manager == FieldManager {
			return true
		}
	}
	return false
}

// childKey identifies a child by group, kind, namespace and name; the
// version is left out so a version change is not a different child.
func childKey(group, kind, namespace, name string) string {
	return group + "/" + kind + "/" + namespace + "/" + name
}
