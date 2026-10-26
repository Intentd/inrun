package housekeeper

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
)

func TestSyncStrictModeWebhook_RestoresDeletedConfig(t *testing.T) {
	h, client := newTestHousekeeper(t, true)

	h.syncStrictModeWebhook()

	cfg, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Get(context.Background(), registration.StrictModeConfigName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("strict-mode configuration not restored: %v", err)
	}
	if got := string(cfg.Webhooks[0].ClientConfig.CABundle); got != "FAKECERT" {
		t.Errorf("caBundle = %q, want the certificate file", got)
	}
	if got := h.Stats().GetStats().Failed; got != 0 {
		t.Errorf("failed = %d, want 0", got)
	}
}

func TestSyncStrictModeWebhook_RemovesConfigWhenOff(t *testing.T) {
	h, client := newTestHousekeeper(t, true)
	h.syncStrictModeWebhook()

	off, _ := newTestHousekeeper(t, false)
	off.client = client
	off.syncStrictModeWebhook()

	_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Get(context.Background(), registration.StrictModeConfigName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("strict-mode configuration still exists with strict mode off")
	}
}
