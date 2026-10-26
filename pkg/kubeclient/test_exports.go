package kubeclient

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// NewForTesting builds a Kubeclient wired to the given REST config without
// going through Start(). Used exclusively by integration tests that provide
// their own envtest config.
func NewForTesting(cfg *rest.Config, dyn dynamic.Interface, scheme *runtime.Scheme) *Kubeclient {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(fmt.Sprintf("NewForTesting: clientset: %v", err))
	}

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		panic(fmt.Sprintf("NewForTesting: discovery client: %v", err))
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	return &Kubeclient{
		name:       "kubeclient-test",
		restConfig: cfg,
		clientset:  clientset,
		dynamic:    dyn,
		mapper:     mapper,
		scheme:     scheme,
	}
}
