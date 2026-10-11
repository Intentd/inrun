// tests/helpers/fake_kubeclient.go
package helpers

import (
	"github.com/intentd/inrun/pkg/kubeclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func NewFakeKubeclient() *kubeclient.Kubeclient {
	return &kubeclient.Kubeclient{
		FakeClientset: fake.NewClientset(),
	}
}

func NewFakeKubeclientWithObjects(objects ...runtime.Object) *kubeclient.Kubeclient {
	return &kubeclient.Kubeclient{
		FakeClientset: fake.NewClientset(objects...),
	}
}
