package kubefake

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Clientset is client-go's fake clientset: a kubernetes.Interface over an
// in-memory tracker, server-side apply included.
type Clientset = k8sfake.Clientset

// Dynamic is client-go's fake dynamic client.
type Dynamic = dynfake.FakeDynamicClient

// Reactor types, for PrependReactor on a Clientset or Dynamic.
type (
	Action           = k8stesting.Action
	CreateAction     = k8stesting.CreateAction
	CreateActionImpl = k8stesting.CreateActionImpl
	DeleteActionImpl = k8stesting.DeleteActionImpl
	PatchActionImpl  = k8stesting.PatchActionImpl
)

// NewClientset returns a fake clientset holding objs.
func NewClientset(objs ...runtime.Object) *Clientset {
	return k8sfake.NewClientset(objs...)
}

// NewDynamic returns a fake dynamic client holding objs. listKinds names the
// List kind of each resource it will be asked to list (the seeds' kinds are
// added); nil is fine when objs cover them.
func NewDynamic(scheme *runtime.Scheme, listKinds map[schema.GroupVersionResource]string, objs ...runtime.Object) *Dynamic {
	if scheme == nil {
		scheme = runtime.NewScheme()
	}
	kinds := ListKindsForSeeds(scheme, objs)
	for gvr, k := range listKinds {
		kinds[gvr] = k
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, kinds, objs...)
}
