package simulate

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/event"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/runtime/reconcilers/remote"
	"github.com/inrundev/inrun/pkg/types"
	"github.com/inrundev/inrun/pkg/utils"
)

// remoteNote tells the user the run depends on their reconciler process.
const remoteNote = "remote reconciler: each cycle calls the endpoint in reconcile.remote — start the reconciler before simulating"

// newRemoteReconciler builds the remote reconciler the runtime would use, over
// kube. Its HTTP calls go to the real endpoint; what it applies lands in kube.
func newRemoteReconciler(crd *types.CRDEntry, kube kubeclient.Interface, ownNamespace string) (domain.Reconciler, error) {
	return remote.New(
		crd.Box().Reconcile.Remote,
		crd.GVK(),
		kube,
		event.Discard(),
		crd.ManagedResources(),
		ownNamespace,
	)
}

// setupObjects reads the CRD's setup.apply files: the objects inrun applies
// before the operator starts, such as a remote reconciler's token Secret.
// The merger has already made the paths absolute.
func setupObjects(crd *types.CRDEntry) ([]*unstructured.Unstructured, error) {
	if crd.Setup == nil {
		return nil, nil
	}
	var objs []*unstructured.Unstructured
	for _, entry := range crd.Setup.Apply {
		data, err := utils.ReadLocal(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("setup %s: %w", entry.Path, err)
		}
		dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			u := &unstructured.Unstructured{}
			if err := dec.Decode(&u.Object); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("setup %s: %w", entry.Path, err)
			}
			if u.GetKind() != "" {
				objs = append(objs, u)
			}
		}
	}
	return objs, nil
}
