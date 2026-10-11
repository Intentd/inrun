// The webhook server's TLS certificate: the one configured in
// security.webhooks, or a self-signed one Inrun generates and keeps in the
// inrun-tls Secret.

package app

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
)

// webhookTLS is the certificate the webhook server serves. mgr and bundle are
// set only when Inrun generated it; a configured certificate leaves them nil,
// and its owner configures the CRDs' spec.conversion.
type webhookTLS struct {
	certFile, keyFile string
	mgr               certmanager.Manager
	bundle            *certmanager.TLSBundle
}

// ensureTLS returns the configured certificate when security.webhooks sets
// both TLS_CERT and TLS_KEY. Otherwise it generates a self-signed one for the
// gateway Service, stores it in the inrun-tls Secret and writes it to files
// for the HTTPS server.
func ensureTLS(ctx context.Context, cfg *config.Config, cat *catalog.Catalog, kube *kubeclient.Kubeclient) (*webhookTLS, error) {
	webhooks := cfg.Security().Webhooks
	if webhooks.TLSCert != "" && webhooks.TLSKey != "" {
		logger.Info().Str("cert", webhooks.TLSCert).Msg("security: using provided TLS certificates")
		return &webhookTLS{certFile: webhooks.TLSCert, keyFile: webhooks.TLSKey}, nil
	}

	logger.Info().Msg("security: generating TLS certificates")
	mgr := certmanager.New(kube.Clientset())
	bundle, err := mgr.EnsureCertificate(ctx, certmanager.CertificateSpec{
		ServiceName: cat.GatewayServiceName(),
		Namespace:   cfg.Cluster().Namespace(),
		SecretName:  certmanager.DefaultTLSSecretName,
		ValidFor:    cat.CertValidForStr(),
		BaseLabels:  labels.InrunResourceLabels(),
	})
	if err != nil {
		return nil, fmt.Errorf("ensuring TLS secret: %w", err)
	}

	certFile, err := writeTemp("inrun-tls-cert-*.pem", bundle.CertPEM)
	if err != nil {
		return nil, fmt.Errorf("writing TLS certificate: %w", err)
	}
	keyFile, err := writeTemp("inrun-tls-key-*.pem", bundle.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("writing TLS key: %w", err)
	}

	logger.Info().
		Str("cert", certFile).
		Str("secret", certmanager.DefaultTLSSecretName).
		Msg("security: TLS certificates generated and stored")
	return &webhookTLS{certFile: certFile, keyFile: keyFile, mgr: mgr, bundle: bundle}, nil
}

// writeTemp writes data to a new temporary file and returns its path.
func writeTemp(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	return f.Name(), f.Close()
}
