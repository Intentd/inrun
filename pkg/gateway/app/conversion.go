// CRD conversion wiring: each CRD whose conversion.manageCRD is in effect is
// pointed at the gateway's /convert with the webhook certificate's CA at
// startup, and patched again by the housekeeper whenever it drifts.

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/housekeeper"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// patchConversionCRDs patches every enabled CRD whose conversion.manageCRD is
// in effect. Each patch is best-effort; errors are collected
// and returned as a combined error so all CRDs are attempted.
func patchConversionCRDs(
	ctx context.Context,
	kube *kubeclient.Kubeclient,
	bp *blueprint.Blueprint,
	caCertPEM []byte,
	serviceName, serviceNamespace string,
	port int32,
) error {
	caBundle64 := base64.StdEncoding.EncodeToString(caCertPEM)

	var errs []error
	for _, crdName := range bp.ManagedConversionCRDs() {
		logger.Info().Str("crd", crdName).Msg("security: patching CRD conversion caBundle")

		if err := applyCRDConversionPatch(ctx, kube, crdName, caBundle64, serviceName, serviceNamespace, port); err != nil {
			errs = append(errs, err)
			continue
		}

		logger.Info().
			Str("crd", crdName).
			Str("service", serviceName+"."+serviceNamespace+".svc").
			Msg("security: CRD conversion webhook patched with caBundle")
	}

	if len(errs) > 0 {
		return fmt.Errorf("crd patch errors: %v", errs)
	}
	return nil
}

// applyCRDConversionPatch applies the conversion webhook caBundle patch to a
// single CRD. Shared by the startup apply (patchConversionCRDs) and the
// housekeeper patcher registered in wireConversion.
func applyCRDConversionPatch(
	ctx context.Context,
	kube *kubeclient.Kubeclient,
	crdName, caBundle64, serviceName, serviceNamespace string,
	port int32,
) error {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"conversion": map[string]interface{}{
				"strategy": "Webhook",
				"webhook": map[string]interface{}{
					"clientConfig": map[string]interface{}{
						"caBundle": caBundle64,
						"service": map[string]interface{}{
							"name":      serviceName,
							"namespace": serviceNamespace,
							"path":      "/convert",
							"port":      port,
						},
					},
					// The ConversionReview API version /convert speaks, not a
					// version of the CRD.
					"conversionReviewVersions": []string{"v1"},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling caBundle patch for %s: %w", crdName, err)
	}

	_, err = kube.ApiextensionsClient().
		ApiextensionsV1().
		CustomResourceDefinitions().
		Patch(ctx, crdName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("patching CRD %s: %w", crdName, err)
	}
	return nil
}

// wireConversion gives the housekeeper what it needs to keep the conversion
// caBundles in sync: the patch, and a watch on each conversion CRD so a
// stripped caBundle is restored at once rather than at the next tick.
func wireConversion(
	hk *housekeeper.Housekeeper,
	kube *kubeclient.Kubeclient,
	bp *blueprint.Blueprint,
	cfg *config.Config,
) {
	serviceName := bp.GatewayServiceName()
	namespace := cfg.Cluster().Namespace()
	port := cfg.HTTPSPortInt32()

	hk.SetConversionCRDPatcher(func(ctx context.Context, crdName, caBundle64 string) error {
		return applyCRDConversionPatch(ctx, kube, crdName, caBundle64, serviceName, namespace, port)
	})

	hk.SetCRDWatcher(&crdWatcher{kube: kube})
}

// crdWatcher implements housekeeper.CRDWatcher using the apiextensions client.
type crdWatcher struct {
	kube *kubeclient.Kubeclient
}

func (c *crdWatcher) Watch(ctx context.Context, crdName string) (watch.Interface, error) {
	return c.kube.ApiextensionsClient().
		ApiextensionsV1().
		CustomResourceDefinitions().
		Watch(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + crdName,
		})
}
