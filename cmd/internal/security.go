// internal/security.go
//
// Security wiring — called from constructRuntime after the HealthServer
// is constructed and before Inrun starts.
//
// Handles:
//
//  1. TLS certificate generation — when deletion protection, admission webhooks,
//     or conversion webhooks are enabled and no explicit cert is configured.
//     Uses certmanager.Manager to generate and store the bundle in inrun-tls Secret.
//
//  2. CRD conversion webhook patch — when a CRD declares conversion.updateCRD: true,
//     Inrun patches the CRD's spec.conversion.webhook.clientConfig.caBundle with
//     the CA certificate from the generated (or configured) TLS bundle.
//
// All operations fatal-log on failure — if security cannot be applied, the
// operator cannot function correctly.

//go:build gateway

package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/intentd/inrun/pkg/gateway/webhook"
)

// ensureSecurity applies TLS certificates when security features
// are enabled in the Catalog. Called synchronously before Start().
//
// Order:
//  1. TLS  — must succeed before the HTTPS server starts (webhook endpoint)
//  2. CRD patch — patches caBundle into CRDs that declare updateCRD: true
//
// Returns the cert file path, key file path, the cert manager (for shutdown cleanup),
// and the TLS bundle (for housekeeper Secret reconciliation).
func ensureSecurity(
	ctx context.Context,
	kfg *config.Config,
	kat *catalog.Catalog,
	kube *kubeclient.Kubeclient,
) (tlsCertFile, tlsKeyFile string, certMgr certmanager.Manager, bundle *certmanager.TLSBundle, err error) {
	namespace := kfg.Cluster().Namespace()

	// ── Namespace labeling ────────────────────────────────────────────────
	// The deletion-protection webhook uses ObjectSelector to narrow to labeled
	// resources. Namespaces are cluster-scoped and carry no labels by default,
	// so the webhook would never fire for the Inrun namespace unless we label
	// it ourselves. Apply the full inrun resource label set (including
	// inrun.dev/deletion-protection) so the webhook intercepts any attempt to
	// delete this namespace.
	// This only makes sense when security.deletionProtection.enabled is true
	// Gating it silences the permissions errors because kat.GenerateRBACRules()
	// does not create namespace permissions if deletionProtection.enabled is false.
	if kat.IsDeletionProtectionEnabled() {
		if err := ensureNamespaceLabeled(ctx, kube, namespace, labels.InrunResourceLabels()); err != nil {
			logger.Warn().Err(err).
				Str("namespace", namespace).
				Msg("security: failed to label inrun namespace — deletion protection will not cover it")
		}
	}

	// ── TLS certificate management ────────────────────────────────────────
	// TLS is required whenever deletion protection, admission webhooks, or
	// conversion webhooks are enabled. If the user has provided TLS_CERT and
	// TLS_KEY those are used as-is. Otherwise Inrun generates self-signed
	// certs and stores them in the inrun-tls Secret.
	needsTLS := kat.NeedsCertificates()
	if !needsTLS {
		return "", "", nil, nil, nil
	}

	configuredCert := kfg.Security().Webhooks.TLSCert
	configuredKey := kfg.Security().Webhooks.TLSKey

	if configuredCert != "" && configuredKey != "" {
		// User provided certs — use them as-is; no CRD patch (user manages caBundle)
		logger.Info().
			Str("cert", configuredCert).
			Msg("security: using provided TLS certificates")
		return configuredCert, configuredKey, nil, nil, nil
	}

	// No explicit cert configured — generate self-signed via certmanager.
	logger.Info().Msg("security: generating TLS certificates")

	serviceName := kat.GatewayServiceName()
	mgr := certmanager.New(kube.Clientset())

	tlsBundle, bundleErr := mgr.EnsureCertificate(ctx, certmanager.CertificateSpec{
		ServiceName: serviceName,
		Namespace:   namespace,
		SecretName:  certmanager.DefaultTLSSecretName,
		ValidFor:    kat.CertValidForStr(),
		BaseLabels:  labels.InrunResourceLabels(),
	})
	if bundleErr != nil {
		logger.Fatal().Err(bundleErr).Msg("security: failed to ensure TLS secret")
	}

	certFile, keyFile, writeErr := writeTLSToFiles(tlsBundle)
	if writeErr != nil {
		logger.Fatal().Err(writeErr).Msg("security: failed to write TLS certificates to files")
	}

	logger.Info().
		Str("cert", certFile).
		Str("secret", certmanager.DefaultTLSSecretName).
		Msg("security: TLS certificates generated and stored")

	// ── 2. CRD conversion webhook patch ──────────────────────────────────────
	// For each enabled CRD that declares conversion.updateCRD: true, patch the
	// CRD's spec.conversion.webhook.clientConfig.caBundle with the generated CA.
	if err := patchConversionCRDs(ctx, kube, kat, tlsBundle.CACertPEM, serviceName, namespace); err != nil {
		logger.Warn().Err(err).Msg("security: some CRD conversion patches failed")
	}

	return certFile, keyFile, mgr, tlsBundle, nil
}

// patchConversionCRDs patches the caBundle into every enabled CRD that has
// conversion.updateCRD: true. Each patch is best-effort; errors are collected
// and returned as a combined error so all CRDs are attempted.
func patchConversionCRDs(
	ctx context.Context,
	kube *kubeclient.Kubeclient,
	kat *catalog.Catalog,
	caCertPEM []byte,
	serviceName, serviceNamespace string,
) error {
	caBundle64 := base64.StdEncoding.EncodeToString(caCertPEM)

	var errs []error
	for _, crd := range kat.EnabledCRDs() {
		if !crd.UpdateCRDCaBundle() {
			continue
		}

		crdName := crd.APITypes.Plural + "." + crd.APITypes.Group
		storageVersion := crd.EffectiveConversion().StorageVersion

		logger.Info().Str("crd", crdName).Str("storageVersion", storageVersion).
			Msg("security: patching CRD conversion caBundle")

		if err := applyCRDConversionPatch(ctx, kube, crdName, caBundle64, storageVersion, serviceName, serviceNamespace); err != nil {
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
// housekeeper patcher registered in WireWebhookHousekeeperInfra.
func applyCRDConversionPatch(
	ctx context.Context,
	kube *kubeclient.Kubeclient,
	crdName, caBundle64, storageVersion, serviceName, serviceNamespace string,
) error {
	port := int32(8443)
	path := "/convert"

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
							"path":      path,
							"port":      port,
						},
					},
					"conversionReviewVersions": []string{storageVersion},
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

// writeTLSToFiles writes the TLS bundle to temporary files.
// Returns the cert file path and key file path.
func writeTLSToFiles(bundle *certmanager.TLSBundle) (certFile, keyFile string, err error) {
	cert, err := os.CreateTemp("", "inrun-tls-cert-*.pem")
	if err != nil {
		return "", "", err
	}
	if _, err := cert.Write(bundle.CertPEM); err != nil {
		return "", "", err
	}
	cert.Close()

	key, err := os.CreateTemp("", "inrun-tls-key-*.pem")
	if err != nil {
		return "", "", err
	}
	if _, err := key.Write(bundle.KeyPEM); err != nil {
		return "", "", err
	}
	key.Close()

	return cert.Name(), key.Name(), nil
}

// ensureNamespaceLabeled patches the Inrun namespace with the given labels
// so that the deletion-protection webhook's ObjectSelector can match it.
// Namespaces are cluster-scoped and carry no labels by default — without this
// patch the webhook rule for namespaces would never fire for this namespace.
// Uses a strategic-merge patch so existing labels are preserved.
func ensureNamespaceLabeled(ctx context.Context, kube *kubeclient.Kubeclient, namespace string, labels map[string]string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling namespace label patch: %w", err)
	}
	_, err = kube.Clientset().CoreV1().Namespaces().Patch(
		ctx, namespace, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("patching namespace %s: %w", namespace, err)
	}
	return nil
}

// WireWebhookHousekeeperInfra provides the WebhookServer with the callbacks it
// needs to keep CRD conversion webhooks correct throughout the deployment
// lifecycle. Called after ensureSecurity and before webhook.Start().
//
// Two hooks are set:
//   - ConversionCRDPatcher — re-patches the caBundle on any CRD with
//     conversion.updateCRD: true when the housekeeper detects drift.
//   - CRDWatcher — watches each conversion CRD for MODIFIED events so the
//     housekeeper restores a stripped caBundle within a single API round-trip
//     rather than waiting for the safety ticker.
func WireWebhookHousekeeperInfra(
	ws *webhook.WebhookServer,
	kube *kubeclient.Kubeclient,
	kat *catalog.Catalog,
	kfg *config.Config,
) {
	serviceName := kat.GatewayServiceName()
	namespace := kfg.Cluster().Namespace()

	ws.SetConversionCRDPatcher(func(ctx context.Context, crdName, caBundle64, storageVersion string) error {
		return applyCRDConversionPatch(ctx, kube, crdName, caBundle64, storageVersion, serviceName, namespace)
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
