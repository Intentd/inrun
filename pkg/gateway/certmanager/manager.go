package certmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultCertValidFor is the default certificate validity duration.
	DefaultCertValidFor = "1y"
)

// DefaultTLSSecretName is the Secret name used for Inrun's auto-generated TLS bundle.
var DefaultTLSSecretName = config.DefaultInternalTLSName()

// CertificateSpec describes the TLS certificate Inrun should generate and store.
type CertificateSpec struct {
	// ServiceName is the Kubernetes Service that will serve the certificate (e.g. "inrun").
	ServiceName string
	// Namespace is the namespace where the Service and Secret live.
	Namespace string
	// SecretName is the name of the Secret to create or update.
	SecretName string
	// ValidFor is the certificate validity duration string ("1y", "90d", etc.).
	ValidFor string
	// BaseLabels are the labels to apply to the Secret in addition to the deletion-protection label.
	BaseLabels map[string]string
}

// Manager handles TLS certificate generation and Secret lifecycle.
type Manager interface {
	// EnsureCertificate generates a TLS bundle and stores it in a Kubernetes Secret.
	// If the Secret already exists it is updated in-place.
	EnsureCertificate(ctx context.Context, spec CertificateSpec) (*TLSBundle, error)
	// DeleteCertificateAndSecret removes the TLS Secret from the cluster.
	// A NotFound error is silently ignored.
	DeleteCertificateAndSecret(ctx context.Context, namespace, secretName string) error
}

type k8sManager struct {
	client kubernetes.Interface
}

// New returns a Manager backed by the given Kubernetes client.
func New(client kubernetes.Interface) Manager {
	return &k8sManager{client: client}
}

// EnsureCertificate generates a self-signed TLS bundle and stores it in a Secret.
// If the Secret already exists and is valid, it is reused without regeneration.
// If the Secret exists but is malformed, it is deleted and a new one is created.
// If the Secret does not exist, a new one is created.
func (m *k8sManager) EnsureCertificate(ctx context.Context, spec CertificateSpec) (*TLSBundle, error) {
	client := m.client.CoreV1().Secrets(spec.Namespace)

	// 1. Try to fetch existing secret
	existing, err := client.Get(ctx, spec.SecretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists – try to extract a valid bundle
		bundle, err := bundleFromSecret(existing)
		if err == nil {
			// Valid certificate – reuse it
			return bundle, nil
		}
		// Secret exists but is malformed – log, delete, and regenerate
		logger.Warn().Err(err).Msg("existing TLS secret invalid, deleting and regenerating")
		if err := client.Delete(ctx, spec.SecretName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("deleting invalid tls secret: %w", err)
		}
	} else if !k8serrors.IsNotFound(err) {
		// Some other error (e.g., network, permissions) – fail fast
		return nil, fmt.Errorf("checking existing tls secret: %w", err)
	}

	// 2. Generate a new TLS bundle (secret does not exist or was deleted)
	bundle, err := GenerateClusterBundle(spec.ServiceName, spec.Namespace, BundleOpts{ValidFor: spec.ValidFor})
	if err != nil {
		return nil, fmt.Errorf("generating tls bundle: %w", err)
	}

	// 3. Prepare the secret object
	secretLabels := labels.WithDeletionProtection(spec.BaseLabels)
	secretLabels["app.kubernetes.io/component"] = "tls"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.SecretName,
			Namespace: spec.Namespace,
			Labels:    secretLabels,
			Annotations: map[string]string{
				"inrun.dev/generated-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": bundle.CertPEM,
			"tls.key": bundle.KeyPEM,
			"ca.crt":  bundle.CACertPEM,
		},
	}

	// 4. Create the secret (idempotent)
	_, err = client.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating tls secret: %w", err)
	}
	return bundle, nil
}

// bundleFromSecret extracts a TLSBundle from a Kubernetes Secret.
// Expects the Secret to have keys "tls.crt", "tls.key", and "ca.crt".
func bundleFromSecret(secret *corev1.Secret) (*TLSBundle, error) {
	certPEM, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, fmt.Errorf("secret missing tls.crt")
	}
	keyPEM, ok := secret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("secret missing tls.key")
	}
	caPEM, ok := secret.Data["ca.crt"]
	if !ok {
		return nil, fmt.Errorf("secret missing ca.crt")
	}
	return &TLSBundle{
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		CACertPEM: caPEM,
	}, nil
}

// DeleteCertificateAndSecret removes the TLS Secret. Silently succeeds if already gone.
func (m *k8sManager) DeleteCertificateAndSecret(ctx context.Context, namespace, secretName string) error {
	err := m.client.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("deleting tls secret %s/%s: %w", namespace, secretName, err)
	}
	return nil
}
