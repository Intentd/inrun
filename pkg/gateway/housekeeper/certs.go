package housekeeper

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// certBundle is the generated TLS bundle, kept to restore its Secret.
type certBundle struct {
	certPEM []byte
	keyPEM  []byte
	caPEM   []byte
}

// SetCertBundle gives the housekeeper the TLS bundle the gateway generated and
// where its Secret lives, so it can restore the Secret if it is deleted. Not
// called when the certificate is configured rather than generated.
func (h *Housekeeper) SetCertBundle(certPEM, keyPEM, caPEM []byte, secretName, namespace string) {
	h.cert = &certBundle{certPEM: certPEM, keyPEM: keyPEM, caPEM: caPEM}
	h.certName = secretName
	h.certNamespace = namespace
}

// syncCertSecret restores the TLS Secret when it is missing and, with
// auto-rotation on, rotates the certificate before it expires.
func (h *Housekeeper) syncCertSecret() {
	if h.cert == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	existing, err := h.client.CoreV1().Secrets(h.certNamespace).Get(ctx, h.certName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error().Err(err).Str("secret", h.certName).Msg("housekeeper: failed to check TLS secret")
			metrics.RecordWebhookReconciliationFailure(labelTLSSecret)
			return
		}
		h.restoreCertSecret(ctx)
		return
	}

	if h.bp.CertAutoRotate() {
		h.maybeRotateCert(ctx, existing)
	}
}

// restoreCertSecret creates the TLS Secret again from the bundle in memory.
func (h *Housekeeper) restoreCertSecret(ctx context.Context) {
	logger.Warn().Str("secret", h.certName).Str("namespace", h.certNamespace).
		Msg("housekeeper: TLS secret missing, restoring from the bundle in memory")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.certName,
			Namespace: h.certNamespace,
			Labels:    labels.WithDeletionProtection(labels.InrunResourceLabels()),
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": h.cert.certPEM,
			"tls.key": h.cert.keyPEM,
			"ca.crt":  h.cert.caPEM,
		},
	}

	if _, err := h.client.CoreV1().Secrets(h.certNamespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			logger.Error().Err(err).Str("secret", h.certName).Msg("housekeeper: failed to restore TLS secret")
			metrics.RecordWebhookReconciliationFailure(labelTLSSecret)
		}
		return
	}

	logger.Info().Str("secret", h.certName).Msg("housekeeper: TLS secret restored")
	metrics.RecordWebhookReconciled(labelTLSSecret)
}

// maybeRotateCert writes a new certificate to the Secret once the stored one
// is within the rotation threshold of expiring. The running server keeps the
// old certificate, which is still valid for the whole threshold; the new one
// takes effect when the gateway restarts.
func (h *Housekeeper) maybeRotateCert(ctx context.Context, existing *corev1.Secret) {
	block, _ := pem.Decode(existing.Data["tls.crt"])
	if block == nil {
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return
	}
	if time.Now().Add(h.bp.CertRotationThreshold()).Before(cert.NotAfter) {
		return
	}

	logger.Warn().Str("secret", h.certName).
		Float64("days_until_expiry", time.Until(cert.NotAfter).Hours()/24).
		Msg("housekeeper: TLS cert within rotation window, rotating")

	bundle, err := certmanager.GenerateClusterBundle(h.bp.GatewayServiceName(), h.certNamespace,
		certmanager.BundleOpts{ValidFor: h.bp.CertValidForStr()})
	if err != nil {
		logger.Error().Err(err).Msg("housekeeper: cert rotation failed to generate a new bundle")
		metrics.RecordWebhookReconciliationFailure(labelTLSRotation)
		return
	}

	// Read it again for the latest resourceVersion, in case it was restored meanwhile.
	fresh, err := h.client.CoreV1().Secrets(h.certNamespace).Get(ctx, h.certName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Warn().Msg("housekeeper: cert rotation skipped, the secret is gone; the next sync restores it")
		} else {
			logger.Error().Err(err).Msg("housekeeper: cert rotation failed to read the secret")
			metrics.RecordWebhookReconciliationFailure(labelTLSRotation)
		}
		return
	}

	fresh.Data["tls.crt"] = bundle.CertPEM
	fresh.Data["tls.key"] = bundle.KeyPEM
	fresh.Data["ca.crt"] = bundle.CACertPEM
	if fresh.Annotations == nil {
		fresh.Annotations = map[string]string{}
	}
	fresh.Annotations["inrun.dev/rotated-at"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := h.client.CoreV1().Secrets(h.certNamespace).Update(ctx, fresh, metav1.UpdateOptions{}); err != nil {
		logger.Error().Err(err).Msg("housekeeper: cert rotation failed to update the secret")
		metrics.RecordWebhookReconciliationFailure(labelTLSRotation)
		return
	}

	// Restore the rotated certificate, not the original, if the Secret is deleted later.
	h.cert = &certBundle{certPEM: bundle.CertPEM, keyPEM: bundle.KeyPEM, caPEM: bundle.CACertPEM}

	logger.Info().Str("secret", h.certName).
		Msg("housekeeper: TLS cert rotated, the new cert takes effect on the next gateway restart")
	metrics.RecordWebhookReconciled(labelTLSRotation)
}
