package kube

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// LeaseOptions names a coordination.k8s.io/v1 Lease and how it is held.
type LeaseOptions struct {
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	// Identity names the holder on the Lease; default LeaseIdentity().
	Identity string
	// Recorder, when set, records the Lease's leader events.
	Recorder resourcelock.EventRecorder

	LeaseDuration time.Duration // how long a holder that stops renewing keeps it
	RenewDeadline time.Duration // how long the holder keeps trying to renew
	RetryPeriod   time.Duration // how often a waiting replica tries to take it
}

// NewLeaseLock is the lock for opts: the runtime's leader election and the
// gateway's startup migration both take their Lease through it.
func NewLeaseLock(cs kubernetes.Interface, opts LeaseOptions) *resourcelock.LeaseLock {
	id := opts.Identity
	if id == "" {
		id = LeaseIdentity()
	}
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:        opts.Name,
			Namespace:   opts.Namespace,
			Labels:      opts.Labels,
			Annotations: opts.Annotations,
		},
		Client: cs.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: opts.Recorder,
		},
	}
}

// LeaseIdentity is this process's name on a Lease: its hostname, which in a
// pod is the pod name, or a random one when there is none.
func LeaseIdentity() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return uuid.New().String()
}

// RunExclusively runs fn while holding the Lease, so the processes that share
// it run fn one at a time: each waits its turn, runs fn, then releases the
// Lease to the next. The Lease is renewed while fn runs; if the holder dies it
// expires after LeaseDuration and the next process takes it. It returns fn's
// error, or ctx's when ctx ends before this process got the Lease.
func RunExclusively(ctx context.Context, cs kubernetes.Interface, opts LeaseOptions, fn func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Name:            opts.Name,
		Lock:            NewLeaseLock(cs, opts),
		LeaseDuration:   opts.LeaseDuration,
		RenewDeadline:   opts.RenewDeadline,
		RetryPeriod:     opts.RetryPeriod,
		ReleaseOnCancel: true, // hand the Lease on as soon as fn is done
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaseCtx context.Context) {
				result <- fn(leaseCtx)
				cancel()
			},
			OnStoppedLeading: func() {},
		},
	})
	if err != nil {
		return fmt.Errorf("lease %s/%s: %w", opts.Namespace, opts.Name, err)
	}
	elector.Run(runCtx) // returns once fn is done and the Lease released, or ctx ends

	select {
	case err := <-result:
		return err
	default:
		if err := ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("lease %s/%s: lost before running", opts.Namespace, opts.Name)
	}
}
