package kube_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/util/kube"
)

// Two replicas start together: they run one at a time, never both at once,
// and the one that waited finds the work done.
func TestRunExclusively_OneAtATime(t *testing.T) {
	cs := kubefake.NewClientset()
	var running, maxRunning, migrated atomic.Int32
	migrate := func(context.Context) error {
		if n := running.Add(1); n > maxRunning.Load() {
			maxRunning.Store(n)
		}
		defer running.Add(-1)
		time.Sleep(200 * time.Millisecond)
		migrated.CompareAndSwap(0, 1) // the first does the work; the next finds it done
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"gateway-a", "gateway-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- kube.RunExclusively(ctx, cs, kube.LeaseOptions{
				Name: "test-lease", Namespace: "inrun-system", Identity: id,
				LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: time.Second,
			}, migrate)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("replica: %v", err)
		}
	}
	if got := maxRunning.Load(); got != 1 {
		t.Fatalf("%d replicas migrated at once, want 1", got)
	}
	if migrated.Load() != 1 {
		t.Fatal("migration never ran")
	}
}
