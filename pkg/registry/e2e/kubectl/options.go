package kubectl

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// BlockOptions is one kubectl: block and what its entries need to reach the
// cluster. Write, Assert and Post all take it.
type BlockOptions struct {
	Kubectl *types.E2EKubectl    // the block whose entries run, in order
	Env     types.EnvResolver    // the expectation's env, in every invocation's environment
	WorkDir string               // cwd for each kubectl call; relative paths resolve here
	Client  kubernetes.Interface // for lease/exec/cp lookups that need the API
	Config  *rest.Config         // for port-forward (SPDY dialer)
	Dynamic dynamic.Interface    // for client-go-native get instead of kubectl subprocess
	Mapper  apimeta.RESTMapper   // resolves kind strings to GVR for the dynamic path
}
