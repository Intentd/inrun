// pkg/kubeclient/genericclient.go
package kubeclient

import (
	"context"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// Client definition
type Client struct {
	restClient rest.Interface
	namespace  string
	plural     string
	codec      runtime.ParameterCodec
	objList    runtime.Object
}

// NewClient returns a new client using ths shared client factory
func (k *Kubeclient) NewClient(objList runtime.Object, info CRDInfo) (*Client, error) {
	restClient, err := k.SharedClientFactory(info.APIPath, info.Group, info.Version)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,
		objList:    objList,
		namespace:  info.Namespace,
		plural:     info.Plural,
		codec:      k.RuntimeParameterCodec(),
	}, nil
}

// A generic client should know how to List and Watch just as the dynamic client
// List returns runtime.Object - exactly what GenericClient needs!
func (c *Client) List(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
	// Create a new instance of the list type
	list := reflect.New(reflect.TypeOf(c.objList).Elem()).Interface().(runtime.Object)

	err := c.restClient.Get().
		Namespace(c.namespace).
		Resource(c.plural).
		VersionedParams(&opts, c.codec).
		Do(ctx).
		Into(list)

	return list, err
}

// Watch returns the watch interface
func (c *Client) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.Get().
		Namespace(c.namespace).
		Resource(c.plural).
		VersionedParams(&opts, c.codec).
		Watch(ctx)
}

// ListInNamespace lists resources scoped to the given namespace.
// Used by the Tier 1 namespace filter when allowedNamespaces has exactly one entry.
func (c *Client) ListInNamespace(ctx context.Context, namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	list := reflect.New(reflect.TypeOf(c.objList).Elem()).Interface().(runtime.Object)
	err := c.restClient.Get().
		Namespace(namespace).
		Resource(c.plural).
		VersionedParams(&opts, c.codec).
		Do(ctx).
		Into(list)
	return list, err
}

// WatchInNamespace watches resources scoped to the given namespace.
// Used by the Tier 1 namespace filter when allowedNamespaces has exactly one entry.
func (c *Client) WatchInNamespace(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.Get().
		Namespace(namespace).
		Resource(c.plural).
		VersionedParams(&opts, c.codec).
		Watch(ctx)
}
