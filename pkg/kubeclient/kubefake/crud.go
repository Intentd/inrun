package kubefake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CRUD stubs — record operations. Get returns the objects the harness seeded
// (the CR under simulation) and NotFound for everything else, so a reconciler
// can read its own CR and still takes the Create path for its children.

func (f *Kubeclient) Get(_ context.Context, key domain.ObjectKey, obj domain.Object, opts metav1.GetOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "get",
		Resource:  resourceNameFromObject(obj),
		Namespace: key.Namespace,
		Name:      key.Name,
		At:        time.Now(),
	})
	f.shared.mu.Unlock()
	if f.getSeeded(key, obj) {
		return nil
	}
	return fakeNotFound(key.Name)
}

// getSeeded copies a seeded object matching obj's kind and key into obj.
func (f *Kubeclient) getSeeded(key domain.ObjectKey, obj domain.Object) bool {
	if f.tracker == nil {
		return false
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		kinds, _, err := f.scheme.ObjectKinds(obj)
		if err != nil || len(kinds) == 0 {
			return false
		}
		gvk = kinds[0]
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	found, err := f.tracker.Get(gvr, key.Namespace, key.Name)
	if err != nil {
		return false
	}
	u, ok := found.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	if dst, ok := obj.(*unstructured.Unstructured); ok {
		dst.Object = u.DeepCopy().Object
		return true
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj) == nil
}

func (f *Kubeclient) Create(_ context.Context, obj domain.Object, opts metav1.CreateOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "create",
		Resource:  resourceNameFromObject(obj),
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		At:        time.Now(),
	})
	f.shared.mu.Unlock()
	return nil
}

func (f *Kubeclient) Update(_ context.Context, obj domain.Object, opts metav1.UpdateOptions) error {
	f.shared.mu.Lock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "update",
		Resource:  resourceNameFromObject(obj),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		At:        time.Now(),
	})
	f.shared.mu.Unlock()
	return nil
}

func (f *Kubeclient) Apply(_ context.Context, cfg runtime.ApplyConfiguration, _ metav1.ApplyOptions) error {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle: f.shared.currentCycle,
		Verb:  "apply",
		At:    time.Now(),
	})
	return nil
}

func (f *Kubeclient) ApplySubResource(_ context.Context, cfg runtime.ApplyConfiguration, subresource string, _ metav1.ApplyOptions) error {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()

	f.shared.ops = append(f.shared.ops, Op{
		Cycle:       f.shared.currentCycle,
		Verb:        "apply",
		Subresource: subresource,
		At:          time.Now(),
	})

	return nil
}

func (f *Kubeclient) Patch(_ context.Context, obj domain.Object, patch kubeclient.Patch, opts metav1.PatchOptions) error {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()

	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "patch",
		Resource:  resourceNameFromObject(obj),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		At:        time.Now(),
	})
	return nil
}

func (f *Kubeclient) Delete(_ context.Context, obj domain.Object, opts metav1.DeleteOptions) error {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "delete",
		Resource:  resourceNameFromObject(obj),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		At:        time.Now(),
	})
	return nil
}

func (f *Kubeclient) DeleteAllOf(_ context.Context, obj domain.Object, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:     f.shared.currentCycle,
		Verb:      "delete",
		Resource:  resourceNameFromObject(obj),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		At:        time.Now(),
	})
	return nil
}

// fakeNotFound returns an error that satisfies k8s.io/apimachinery/pkg/api/errors.IsNotFound.
func fakeNotFound(name string) error {
	return k8serrors.NewNotFound(schema.GroupResource{}, name)
}

func resourceNameFromObject(obj runtime.Object) string {
	t := fmt.Sprintf("%T", obj)
	if idx := strings.LastIndex(t, "."); idx >= 0 {
		t = t[idx+1:]
	}
	return strings.ToLower(t) + "s"
}

func (f *Kubeclient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return kubeclient.GroupVersionKindFor(f, obj)
}

func (f *Kubeclient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return kubeclient.IsObjectNamespaced(f, obj)
}

// compile-time check
var _ kubeclient.Interface = (*Kubeclient)(nil)

// Subresource stubs — record writes, as the controller-runtime adapter's
// Status() and SubResource() use them.

var _ kubeclient.Subresource = (*Kubeclient)(nil)

func (f *Kubeclient) GetSubResource(_ context.Context, obj domain.Object, _ domain.Object, subresource string, _ metav1.GetOptions) error {
	return fakeNotFound(obj.GetName())
}

func (f *Kubeclient) CreateSubResource(_ context.Context, obj domain.Object, _ domain.Object, subresource string, _ metav1.CreateOptions) error {
	f.recordSubresource("create", obj, subresource)
	return nil
}

func (f *Kubeclient) UpdateSubResource(_ context.Context, obj domain.Object, subresource string, _ metav1.UpdateOptions) error {
	f.recordSubresource("update", obj, subresource)
	return nil
}

func (f *Kubeclient) PatchSubResource(_ context.Context, obj domain.Object, _ kubeclient.Patch, subresource string, _ metav1.PatchOptions) error {
	f.recordSubresource("patch", obj, subresource)
	return nil
}

func (f *Kubeclient) recordSubresource(verb string, obj domain.Object, subresource string) {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.ops = append(f.shared.ops, Op{
		Cycle:       f.shared.currentCycle,
		Verb:        verb,
		Resource:    resourceNameFromObject(obj),
		Subresource: subresource,
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		At:          time.Now(),
	})
}
