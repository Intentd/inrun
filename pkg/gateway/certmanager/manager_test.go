package certmanager_test

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/gateway/certmanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newManager(t *testing.T) (certmanager.Manager, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset()
	return certmanager.New(cs), cs
}

func TestEnsureCertificate_Creates(t *testing.T) {
	mgr, cs := newManager(t)
	ctx := context.Background()

	spec := certmanager.CertificateSpec{
		ServiceName: "inrun",
		Namespace:   "inrun-system",
		SecretName:  certmanager.DefaultTLSSecretName,
		ValidFor:    "1y",
		BaseLabels: map[string]string{
			"app.kubernetes.io/name": "inrun",
		},
	}

	bundle, err := mgr.EnsureCertificate(ctx, spec)
	if err != nil {
		t.Fatalf("EnsureCertificate: %v", err)
	}
	if len(bundle.CertPEM) == 0 {
		t.Fatal("expected non-empty CertPEM")
	}

	secret, err := cs.CoreV1().Secrets(spec.Namespace).Get(ctx, spec.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting created secret: %v", err)
	}
	if secret.Type != corev1.SecretTypeTLS {
		t.Errorf("expected TLS type, got %s", secret.Type)
	}
	if secret.Labels["inrun.dev/deletion-protection"] != "true" {
		t.Error("deletion-protection label missing")
	}
	if secret.Labels["app.kubernetes.io/component"] != "tls" {
		t.Error("component label missing")
	}
}

func TestEnsureCertificate_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            certmanager.DefaultTLSSecretName,
			Namespace:       "inrun-system",
			ResourceVersion: "1",
		},
		Type: corev1.SecretTypeTLS,
	}
	cs := fake.NewClientset(existing)
	mgr := certmanager.New(cs)

	spec := certmanager.CertificateSpec{
		ServiceName: "inrun",
		Namespace:   "inrun-system",
		SecretName:  certmanager.DefaultTLSSecretName,
		ValidFor:    "1y",
	}

	bundle, err := mgr.EnsureCertificate(ctx, spec)
	if err != nil {
		t.Fatalf("EnsureCertificate on existing: %v", err)
	}
	if len(bundle.CACertPEM) == 0 {
		t.Fatal("expected non-empty CACertPEM on update")
	}
}

func TestDeleteCertificateAndSecret(t *testing.T) {
	ctx := context.Background()
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certmanager.DefaultTLSSecretName,
			Namespace: "inrun-system",
		},
	}
	cs := fake.NewClientset(existing)
	mgr := certmanager.New(cs)

	if err := mgr.DeleteCertificateAndSecret(ctx, "inrun-system", certmanager.DefaultTLSSecretName); err != nil {
		t.Fatalf("DeleteCertificateAndSecret: %v", err)
	}

	// Verify gone.
	_, err := cs.CoreV1().Secrets("inrun-system").Get(ctx, certmanager.DefaultTLSSecretName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected secret to be deleted")
	}
}

func TestDeleteCertificateAndSecret_NotFound(t *testing.T) {
	mgr, _ := newManager(t)
	// Should not return an error when the secret doesn't exist.
	err := mgr.DeleteCertificateAndSecret(context.Background(), "ns", "missing-secret")
	if err != nil {
		t.Fatalf("expected no error on not-found, got: %v", err)
	}
}
