// pkg/kubeclient/dynamic.go
package kubeclient

import (
	"context"

	"github.com/intentd/inrun/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// DynamicListerWatcher implements cache.ListerWatcher using the dynamic client.
// Safe to construct before kube.Start() — k.dynamic is resolved at call time.
type DynamicListerWatcher struct {
	kube          *Kubeclient
	gvr           schema.GroupVersionResource
	namespace     string
	namespaced    bool
	labelSelector string
	fieldSelector string
}

type ListOptions struct {
	LabelSelector string
	FieldSelector string
}

// NewDynamicListerWatcher builds a cache.ListerWatcher for an unstructured CRD.
// Safe to call before kube.Start() — dynamic client is resolved lazily.
func (k *Kubeclient) NewDynamicListerWatcher(info CRDInfo, opts ListOptions) cache.ListerWatcher {
	return &DynamicListerWatcher{
		kube: k,
		gvr: schema.GroupVersionResource{
			Group:    info.Group,
			Version:  info.Version,
			Resource: info.Plural,
		},
		namespace:     info.Namespace,
		namespaced:    info.Namespaced,
		labelSelector: opts.LabelSelector,
		fieldSelector: opts.FieldSelector,
	}
}

// List implements cache.ListerWatcher.
// *unstructured.UnstructuredList satisfies runtime.Object — explicit cast needed
// because dynamic.ResourceInterface.List returns the concrete type, not the interface.
func (d *DynamicListerWatcher) List(options metav1.ListOptions) (runtime.Object, error) {

	// Inject label selector
	if d.labelSelector != "" {
		utils.Merge(&options.LabelSelector, d.labelSelector, ",")
	}
	// Inject field selector
	if d.fieldSelector != "" {
		utils.Merge(&options.FieldSelector, d.fieldSelector, ",")
	}

	// Namespaced
	if d.namespaced {
		ns := d.namespace
		if ns == "" {
			ns = metav1.NamespaceAll
		}
		return d.kube.dynamic.Resource(d.gvr).Namespace(ns).List(context.Background(), options)
	}

	// Cluster-scoped
	return d.kube.dynamic.Resource(d.gvr).List(context.Background(), options)
}

// Watch implements cache.ListerWatcher.
func (d *DynamicListerWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {

	// Inject label selector
	if d.labelSelector != "" {
		utils.Merge(&options.LabelSelector, d.labelSelector, ",")
	}
	// Inject field selector
	if d.fieldSelector != "" {
		utils.Merge(&options.FieldSelector, d.fieldSelector, ",")
	}

	// Namespaced
	if d.namespaced {
		ns := d.namespace
		if ns == "" {
			ns = metav1.NamespaceAll
		}
		return d.kube.dynamic.Resource(d.gvr).Namespace(ns).Watch(context.Background(), options)
	}

	// Cluster-scoped
	return d.kube.dynamic.Resource(d.gvr).Watch(context.Background(), options)
}
