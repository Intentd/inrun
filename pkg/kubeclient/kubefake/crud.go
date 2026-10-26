package kubefake

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// CRUD on typed objects — the controller-runtime adapter's client. Each call
// goes through the fake dynamic client, as the real kubeclient's go to the
// API server: it is recorded by its resource and group, and it reads and
// writes the same store as every other path (see store.go), so a child
// created here is found by the next Get, and by .children. Like a real
// client, a call that returns an object writes it back into obj.

// resourceClient returns the dynamic client for obj's resource, in its
// namespace.
func (f *Kubeclient) resourceClient(obj runtime.Object) (dynamic.ResourceInterface, error) {
	gvr, _, err := f.resourceOf(obj)
	if err != nil {
		return nil, err
	}
	var ns string
	if acc, ok := obj.(interface{ GetNamespace() string }); ok {
		ns = acc.GetNamespace()
	}
	if ns == "" {
		return f.dynamic.Resource(gvr), nil
	}
	return f.dynamic.Resource(gvr).Namespace(ns), nil
}

// unstructuredOf returns obj as unstructured, with its apiVersion and kind.
func (f *Kubeclient) unstructuredOf(obj runtime.Object) (*unstructured.Unstructured, error) {
	_, gvk, err := f.resourceOf(obj)
	if err != nil {
		return nil, err
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		cp := u.DeepCopy()
		cp.SetGroupVersionKind(gvk)
		return cp, nil
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetGroupVersionKind(gvk)
	return u, nil
}

// writeBack copies the object a call returned into obj.
func writeBack(u *unstructured.Unstructured, obj runtime.Object) error {
	if u == nil {
		return nil
	}
	if dst, ok := obj.(*unstructured.Unstructured); ok {
		dst.Object = u.DeepCopy().Object
		return nil
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
}

func (f *Kubeclient) Get(ctx context.Context, key domain.ObjectKey, obj domain.Object, opts metav1.GetOptions) error {
	gvr, _, err := f.resourceOf(obj)
	if err != nil {
		return err
	}
	rc := dynamic.ResourceInterface(f.dynamic.Resource(gvr))
	if key.Namespace != "" {
		rc = f.dynamic.Resource(gvr).Namespace(key.Namespace)
	}
	u, err := rc.Get(ctx, key.Name, opts)
	if err != nil {
		return err
	}
	return writeBack(u, obj)
}

func (f *Kubeclient) Create(ctx context.Context, obj domain.Object, opts metav1.CreateOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	u, err := f.unstructuredOf(obj)
	if err != nil {
		return err
	}
	out, err := rc.Create(ctx, u, opts)
	if err != nil {
		return err
	}
	return writeBack(out, obj)
}

func (f *Kubeclient) Update(ctx context.Context, obj domain.Object, opts metav1.UpdateOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	u, err := f.unstructuredOf(obj)
	if err != nil {
		return err
	}
	out, err := rc.Update(ctx, u, opts)
	if err != nil {
		return err
	}
	return writeBack(out, obj)
}

// Apply and ApplySubResource take an apply configuration, which names no
// resource the fake can resolve; they are recorded and not stored.
func (f *Kubeclient) Apply(_ context.Context, cfg runtime.ApplyConfiguration, _ metav1.ApplyOptions) error {
	f.recordUnresolved("apply", "")
	return nil
}

func (f *Kubeclient) ApplySubResource(_ context.Context, cfg runtime.ApplyConfiguration, subresource string, _ metav1.ApplyOptions) error {
	f.recordUnresolved("apply", subresource)
	return nil
}

func (f *Kubeclient) recordUnresolved(verb, subresource string) {
	f.shared.mu.Lock()
	defer f.shared.mu.Unlock()
	f.shared.ops = append(f.shared.ops, Op{Cycle: f.shared.currentCycle, Verb: verb, Subresource: subresource})
}

func (f *Kubeclient) Patch(ctx context.Context, obj domain.Object, patch kubeclient.Patch, opts metav1.PatchOptions) error {
	return f.patch(ctx, obj, patch, opts)
}

func (f *Kubeclient) patch(ctx context.Context, obj domain.Object, patch kubeclient.Patch, opts metav1.PatchOptions, subresources ...string) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}
	out, err := rc.Patch(ctx, obj.GetName(), patch.Type(), data, opts, subresources...)
	if err != nil {
		return err
	}
	return writeBack(out, obj)
}

func (f *Kubeclient) Delete(ctx context.Context, obj domain.Object, opts metav1.DeleteOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	return rc.Delete(ctx, obj.GetName(), opts)
}

func (f *Kubeclient) DeleteAllOf(ctx context.Context, obj domain.Object, opts metav1.DeleteOptions, list metav1.ListOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	return rc.DeleteCollection(ctx, opts, list)
}

// fakeNotFound returns an error that satisfies k8s.io/apimachinery/pkg/api/errors.IsNotFound.
func fakeNotFound(name string) error {
	return k8serrors.NewNotFound(schema.GroupResource{}, name)
}

func (f *Kubeclient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return kubeclient.GroupVersionKindFor(f, obj)
}

func (f *Kubeclient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return kubeclient.IsObjectNamespaced(f, obj)
}

// compile-time check
var _ kubeclient.Interface = (*Kubeclient)(nil)

// Subresource writes — the controller-runtime adapter's Status() and
// SubResource() — go through the dynamic client like the rest, so they are
// recorded by the object's resource, as the envtest recorder records the
// same request, and stored.

var _ kubeclient.Subresource = (*Kubeclient)(nil)

func (f *Kubeclient) GetSubResource(ctx context.Context, obj domain.Object, sub domain.Object, subresource string, opts metav1.GetOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	u, err := rc.Get(ctx, obj.GetName(), opts, subresource)
	if err != nil {
		return err
	}
	return writeBack(u, sub)
}

func (f *Kubeclient) CreateSubResource(ctx context.Context, obj domain.Object, sub domain.Object, subresource string, opts metav1.CreateOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	u, err := f.unstructuredOf(sub)
	if err != nil {
		return err
	}
	out, err := rc.Create(ctx, u, opts, subresource)
	if err != nil {
		return err
	}
	return writeBack(out, sub)
}

func (f *Kubeclient) UpdateSubResource(ctx context.Context, obj domain.Object, subresource string, opts metav1.UpdateOptions) error {
	rc, err := f.resourceClient(obj)
	if err != nil {
		return err
	}
	u, err := f.unstructuredOf(obj)
	if err != nil {
		return err
	}
	out, err := rc.Update(ctx, u, opts, subresource)
	if err != nil {
		return err
	}
	return writeBack(out, obj)
}

func (f *Kubeclient) PatchSubResource(ctx context.Context, obj domain.Object, patch kubeclient.Patch, subresource string, opts metav1.PatchOptions) error {
	return f.patch(ctx, obj, patch, opts, subresource)
}
