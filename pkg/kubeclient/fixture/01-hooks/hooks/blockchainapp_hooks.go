package hooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/intentd/inrun-args-hooks/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/deployments"
)

// BlockchainAppHooks returns the hook implementation registered in the Blueprint.
func BlockchainAppHooks() domain.AnyReconcileHooks {
	return domain.ReconcileHooks[*apiv1.BlockchainApp]{OnReconcile: onBlockchainAppReconcile}
}

func onBlockchainAppReconcile(ctx context.Context, req domain.Request, obj *apiv1.BlockchainApp) (domain.Result, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return domain.Result{}, fmt.Errorf("kubeclient not in context")
	}

	// Both values come from the Blueprint — not from the CR spec.
	// inBusinessHours: computed from a user-defined note (world state).
	// featureEnabled:  result of an external HTTP call the runtime made,
	//                  but only when inBusinessHours was true (see blueprint.yaml when:).
	// The hook binary never changes when the schedule or flag endpoint changes.
	inBusinessHours := req.Args().String("inBusinessHours") == "true"
	featureEnabled := req.Args().String("featureEnabled") == "true"

	annotation := "false"
	if inBusinessHours && featureEnabled {
		annotation = "true"
	}

	replicas := int32(obj.Spec.Replicas)
	if replicas == 0 {
		replicas = 1
	}

	spec := deployments.Spec{
		Name:      obj.Name,
		Namespace: obj.Namespace,
		Image:     obj.Spec.Image,
		Replicas:  replicas,
		Annotations: map[string]string{
			"feature.demo/v2-enabled": annotation,
		},
	}
	if _, err := deployments.Apply(ctx, kube.Clientset(), obj, spec, kubeclient.ApplyOptions(kube, spec.ForceConflict)); err != nil {
		return domain.Result{}, fmt.Errorf("blockchainapp deployment: %w", err)
	}

	return domain.Result{}, kube.PatchStatus(ctx, obj, map[string]any{
		"featureEnabled":  annotation,
		"inBusinessHours": inBusinessHours,
	}, metav1.PatchOptions{})
}
