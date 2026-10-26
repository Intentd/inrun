package registration

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRegisterAdmissionWebhooks_CreatesValidatingAndMutating(t *testing.T) {
	ctx := context.Background()

	// Fake CA bundle
	tmpCert := filepath.Join(t.TempDir(), "tls.crt")
	if err := os.WriteFile(tmpCert, []byte("FAKECERT"), 0600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}

	// Fake admission registry
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

	opts := Options{
		ServiceName:      "inrun",
		ServiceNamespace: "default",
		Port:             8443,
		FailurePolicy:    admissionv1.Ignore,
		TLSCertFile:      tmpCert,
	}

	client := kubefake.NewClientset()

	// Register
	if err := RegisterAdmissionWebhooks(ctx, client, reg, opts); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Validate: ValidatingWebhookConfiguration exists
	vwc, err := client.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, ValidatingConfigName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("validating webhook missing: %v", err)
	}

	if len(vwc.Webhooks) != 1 {
		t.Fatalf("expected 1 validating webhook, got %d", len(vwc.Webhooks))
	}

	// Validate: MutatingWebhookConfiguration exists
	mwc, err := client.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(ctx, MutatingConfigName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("mutating webhook missing: %v", err)
	}

	if len(mwc.Webhooks) != 1 {
		t.Fatalf("expected 1 mutating webhook, got %d", len(mwc.Webhooks))
	}

	// Validate service reference
	vsvc := vwc.Webhooks[0].ClientConfig.Service
	if vsvc.Name != "inrun" || vsvc.Namespace != "default" {
		t.Fatalf("unexpected service reference: %+v", vsvc)
	}

	// Validate CA bundle
	if string(vwc.Webhooks[0].ClientConfig.CABundle) != "FAKECERT" {
		t.Fatalf("CA bundle mismatch")
	}

	// Validate rules
	if len(vwc.Webhooks[0].Rules) != 1 {
		t.Fatalf("expected 1 rule in validating webhook")
	}

	rule := vwc.Webhooks[0].Rules[0]
	if rule.APIGroups[0] != "demo.inrun.dev" ||
		rule.APIVersions[0] != "v1alpha1" ||
		rule.Resources[0] != "websites" {
		t.Fatalf("unexpected rule: %+v", rule)
	}

	// Idempotency: calling again should not error
	if err := RegisterAdmissionWebhooks(ctx, client, reg, opts); err != nil {
		t.Fatalf("idempotent registration failed: %v", err)
	}
}
