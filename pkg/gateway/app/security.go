// Security setup for the in-cluster gateway, called from New after the
// kubeclient starts and before the gateway does. ensureSecurity runs the
// steps in order: label the Inrun namespace, get the webhook certificate
// (tls.go), then patch its CA into conversion CRDs (conversion.go).

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ensureSecurity prepares what the webhook server needs before it starts. It
// returns the certificate to serve, or nil when no webhook needs one.
//
//  1. Namespace label — only with deletion protection, whose webhook selects
//     labeled objects; the namespace carries no labels by default. Without
//     deletion protection the RBAC rules grant no namespace access either.
//  2. TLS — needed for deletion protection, admission or conversion webhooks.
//  3. Conversion — for a generated certificate, every CRD that should be
//     managed (conversion.manageCRD) is pointed at the gateway with its CA.
//     A configured certificate's owner configures spec.conversion.
//
// The label and the CRD patches are best-effort and only logged on failure;
// a certificate error is returned.
func ensureSecurity(ctx context.Context, cfg *config.Config, bp *blueprint.Blueprint, kube *kubeclient.Kubeclient) (*webhookTLS, error) {
	namespace := cfg.Cluster().Namespace()

	if bp.IsDeletionProtectionEnabled() {
		if err := ensureNamespaceLabeled(ctx, kube, namespace, labels.InrunResourceLabels()); err != nil {
			logger.Warn().Err(err).
				Str("namespace", namespace).
				Msg("security: failed to label inrun namespace — deletion protection will not cover it")
		}
	}

	if !bp.NeedsCertificates() {
		return nil, nil
	}
	tls, err := ensureTLS(ctx, cfg, bp, kube)
	if err != nil {
		return nil, err
	}

	if tls.bundle != nil {
		if err := patchConversionCRDs(ctx, kube, bp, tls.bundle.CACertPEM,
			bp.GatewayServiceName(), namespace, cfg.HTTPSPortInt32()); err != nil {
			logger.Warn().Err(err).Msg("security: some CRD conversion patches failed")
		}
	}
	return tls, nil
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
