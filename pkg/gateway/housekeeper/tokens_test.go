package housekeeper

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/config"
)

func TestSyncTokenSecrets_ReloadsWhenSecretMissing(t *testing.T) {
	h, _ := newTestHousekeeper(t, false)
	reloads := 0
	h.SetTokenReloader(func(context.Context) error { reloads++; return nil })

	h.syncTokenSecrets()

	if reloads != 1 {
		t.Errorf("reloads = %d, want 1", reloads)
	}
}

func TestSyncTokenSecrets_NoReloadWhenSecretPresent(t *testing.T) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      "console-token",
		Namespace: config.NewDefaultConfig().Cluster().Namespace(),
	}}
	h, _ := newTestHousekeeper(t, false, secret)
	reloads := 0
	h.SetTokenReloader(func(context.Context) error { reloads++; return nil })

	h.syncTokenSecrets()

	if reloads != 0 {
		t.Errorf("reloads = %d, want 0", reloads)
	}
}
