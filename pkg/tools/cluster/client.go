package cluster

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"k8s.io/client-go/tools/clientcmd"
)

// LocalClient builds a kubeclient from the local kubeconfig.
// When kubectx is empty the current context is used.
func LocalClient(ctx context.Context, kubectx string) (kubeclient.Interface, error) {
	overrides := &clientcmd.ConfigOverrides{}
	if kubectx != "" {
		overrides.CurrentContext = kubectx
	}
	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), overrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}
	return kubeclient.NewFromConfig(ctx, restCfg, nil)
}
