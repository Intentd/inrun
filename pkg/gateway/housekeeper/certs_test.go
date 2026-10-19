package housekeeper

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncCertSecret_RestoresDeletedSecret(t *testing.T) {
	h, client := newTestHousekeeper(t, false)
	h.SetCertBundle([]byte("cert"), []byte("key"), []byte("ca"), "inrun-tls", "inrun-system")

	h.syncCertSecret()

	s, err := client.CoreV1().Secrets("inrun-system").Get(context.Background(), "inrun-tls", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret not restored: %v", err)
	}
	if string(s.Data["tls.crt"]) != "cert" || string(s.Data["tls.key"]) != "key" || string(s.Data["ca.crt"]) != "ca" {
		t.Errorf("restored secret has the wrong data")
	}
	if s.Type != corev1.SecretTypeTLS {
		t.Errorf("type = %s, want %s", s.Type, corev1.SecretTypeTLS)
	}
}

func TestSyncCertSecret_NothingToDoForConfiguredCert(t *testing.T) {
	h, client := newTestHousekeeper(t, false)

	h.syncCertSecret()

	if n := len(client.Actions()); n != 0 {
		t.Errorf("made %d API calls without a generated bundle, want 0", n)
	}
}
