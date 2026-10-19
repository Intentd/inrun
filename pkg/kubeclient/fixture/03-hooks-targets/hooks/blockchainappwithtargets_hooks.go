package hooks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/intentd/inrun-args-hooks-targets/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/kuberesources/core/deployments"
)

// BlockchainAppHooks returns the hook implementation registered in the Blueprint.
func BlockchainAppHooks() domain.AnyReconcileHooks {
	return domain.ReconcileHooks[*apiv1.BlockchainAppWithTargets]{OnReconcile: onBlockchainAppWithTargetsReconcile}
}

func onBlockchainAppWithTargetsReconcile(ctx context.Context, req domain.Request, obj *apiv1.BlockchainAppWithTargets) (domain.Result, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return domain.Result{}, fmt.Errorf("kubeclient not in context")
	}

	// Both values come from the Blueprint args — resolved per target surface.
	// v2-enabled target: featureEnabled="true", enqueueGate blocks outside hours.
	// v2-disabled target: featureEnabled="false", no gate.
	// The hook binary is identical — only the args change between targets.
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
		return domain.Result{}, fmt.Errorf("blockchainappwithtargets deployment: %w", err)
	}

	return domain.Result{}, kube.PatchStatus(ctx, obj, map[string]any{
		"featureEnabled":  annotation,
		"inBusinessHours": inBusinessHours,
	}, metav1.PatchOptions{})
}
