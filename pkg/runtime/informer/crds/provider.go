package crds

import (
	"fmt"
	"reflect"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"k8s.io/apimachinery/pkg/runtime"
)

// ClientProvider definition for registering new clients
type ClientProvider struct {
	kube    *kubeclient.Kubeclient
	clients map[reflect.Type]ClientFactory
}

// ClientFactory accepts kubeclient and returns a generic client
// The generic client is the hallmark of the whole design
// constructRuntime() performs per CRD registration and
// hands over to ghe informer factory
type ClientFactory func(*kubeclient.Kubeclient) (informer.GenericClient, error)

// NewClientProvider creates a new provider with the passed in kubeclient
// At this stage, it only accepts the kubeclient and created the clients map
// This is because kube is not live yet
func NewClientProvider(k *kubeclient.Kubeclient) *ClientProvider {
	return &ClientProvider{
		kube:    k,
		clients: make(map[reflect.Type]ClientFactory),
	}
}

// Register adds a new object to clients
func (p *ClientProvider) Register(obj runtime.Object, factory ClientFactory) {
	p.clients[reflect.TypeOf(obj)] = factory
}

// For returns a generic client for a registered object
func (p *ClientProvider) For(obj runtime.Object) (informer.GenericClient, error) {
	factory, ok := p.clients[reflect.TypeOf(obj)]
	if !ok {
		return nil, fmt.Errorf("no client registered for %T", obj)
	}
	return factory(p.kube)
}
