//go:build integration

package health_test

import (
	"context"
	"os"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/types"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestWebhookRegistration_Integration(t *testing.T) {
	ctx := context.Background()

	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("envtest start failed: %v", err)
	}
	defer testEnv.Stop() //nolint:errcheck

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("client init failed: %v", err)
	}

	certFile, err := os.CreateTemp(t.TempDir(), "tls-*.crt")
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if _, err := certFile.WriteString("FAKECERT"); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	certFile.Close()

	gvr := catalog.GVREntry{
		Key:        "demo.inrun.dev/v1alpha1/websites",
		Group:      "demo.inrun.dev",
		Version:    "v1alpha1",
		Resource:   "websites",
		Operations: []string{"CREATE", "UPDATE"},
	}

	reg := catalog.NewInMemoryAdmissionRegistry()
	reg.AddValidationGVR(gvr, &types.ValidationConfig{})
	reg.AddMutationGVR(gvr, &types.MutationConfig{})

	opts := webhook.WebhookRegistrationOptions{
		ServiceName:      "inrun",
		ServiceNamespace: "default",
		Port:             8443,
		FailurePolicy:    admissionv1.Ignore,
		TLSCertFile:      certFile.Name(),
	}

	if err := webhook.RegisterAdmissionWebhooks(ctx, client, reg, opts); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	vwc, err := client.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, "inrun-admission-validation", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("validating webhook missing: %v", err)
	}
	if len(vwc.Webhooks) != 1 {
		t.Fatalf("expected 1 validating webhook, got %d", len(vwc.Webhooks))
	}

	mwc, err := client.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(ctx, "inrun-admission-mutation", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("mutating webhook missing: %v", err)
	}
	if len(mwc.Webhooks) != 1 {
		t.Fatalf("expected 1 mutating webhook, got %d", len(mwc.Webhooks))
	}

	// Idempotent: calling again must not error
	if err := webhook.RegisterAdmissionWebhooks(ctx, client, reg, opts); err != nil {
		t.Fatalf("idempotent registration failed: %v", err)
	}
}
