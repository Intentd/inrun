// CRD conversion wiring: each CRD whose conversion.manageCRD is in effect is
// pointed at the gateway's /convert with the webhook certificate's CA at
// startup, and patched again by the webhook server's housekeeper whenever it
// drifts.

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/webhook"
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
	cat *catalog.Catalog,
	caCertPEM []byte,
	serviceName, serviceNamespace string,
	port int32,
) error {
	caBundle64 := base64.StdEncoding.EncodeToString(caCertPEM)

	var errs []error
	for _, crd := range cat.EnabledCRDs() {
		if !crd.ShouldManageCRD() {
			continue
		}

		crdName := crd.APITypes.Plural + "." + crd.APITypes.Group
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

// wireConversion gives the WebhookServer's housekeeper the callbacks it needs
// to keep CRD conversion webhooks correct for the life of the deployment.
// Called after ensureSecurity and before webhook.Start().
//
// Two hooks are set:
//   - ConversionCRDPatcher — re-patches any CRD whose
//     conversion.manageCRD is in effect when the housekeeper detects drift.
//   - CRDWatcher — watches each conversion CRD for MODIFIED events so the
//     housekeeper restores a stripped caBundle within a single API round-trip
//     rather than waiting for the safety ticker.
func wireConversion(
	ws *webhook.WebhookServer,
	kube *kubeclient.Kubeclient,
	cat *catalog.Catalog,
	cfg *config.Config,
) {
	serviceName := cat.GatewayServiceName()
	namespace := cfg.Cluster().Namespace()
	port := cfg.HTTPSPortInt32()

	ws.SetConversionCRDPatcher(func(ctx context.Context, crdName, caBundle64 string) error {
		return applyCRDConversionPatch(ctx, kube, crdName, caBundle64, serviceName, namespace, port)
	})

	ws.SetCRDWatcher(&crdWatcher{kube: kube})
}

// crdWatcher implements webhook.CRDWatcher using the apiextensions client.
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
