package housekeeper

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestWatchLoop_TriggersOnlyOnItsEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := watch.NewFake()
	trigger := make(chan struct{}, 1)
	go watchLoop(ctx, trigger, "test", watch.Deleted, func(context.Context) (watch.Interface, error) {
		return w, nil
	})

	w.Modify(&corev1.Secret{})
	select {
	case <-trigger:
		t.Fatal("MODIFIED triggered a sync")
	case <-time.After(50 * time.Millisecond):
	}

	w.Delete(&corev1.Secret{})
	select {
	case <-trigger:
	case <-time.After(time.Second):
		t.Fatal("DELETED did not trigger a sync")
	}
}
