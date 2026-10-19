//go:build integration

package health_test

import (
	"context"
	"os"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/types"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestWebhookLifecycle_Integration(t *testing.T) {
	ctx := context.Background()

	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start envtest: %v", err)
	}
	defer testEnv.Stop() //nolint:errcheck

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	certFile, err := os.CreateTemp(t.TempDir(), "tls-*.crt")
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if _, err := certFile.WriteString("FAKECERT"); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	certFile.Close()

	gvr := blueprint.GVREntry{
		Key:        "demo.inrun.dev/v1alpha1/websites",
		Group:      "demo.inrun.dev",
		Version:    "v1alpha1",
		Resource:   "websites",
		Operations: []string{"CREATE", "UPDATE"},
	}

	reg := blueprint.NewInMemoryAdmissionRegistry()
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
		t.Fatalf("register failed: %v", err)
	}

	if _, err := client.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, "inrun-admission-validation", metav1.GetOptions{}); err != nil {
		t.Fatalf("validating webhook missing: %v", err)
	}

	if _, err := client.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(ctx, "inrun-admission-mutation", metav1.GetOptions{}); err != nil {
		t.Fatalf("mutating webhook missing: %v", err)
	}

	if err := webhook.UnregisterAdmissionWebhooks(ctx, client, webhook.CleanupAllWebhooks()); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if _, err := client.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, "inrun-admission-validation", metav1.GetOptions{}); err == nil {
		t.Fatal("validating webhook still exists after unregister")
	}

	if _, err := client.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(ctx, "inrun-admission-mutation", metav1.GetOptions{}); err == nil {
		t.Fatal("mutating webhook still exists after unregister")
	}
}
