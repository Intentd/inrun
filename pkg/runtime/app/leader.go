package app

import (
	"context"
	"sync/atomic"
	"time"

	kubeutil "github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type LeaderElection struct {
	name       string
	kube       *kubeclient.Kubeclient
	event      *event.Event
	cancelFunc context.CancelFunc
	runCancel  context.CancelFunc
	run        func(context.Context)

	election theElection
	opts     LeaderOptions
	started  bool
}

type theElection struct {
	startedLeading atomic.Bool
	leader         string
	onElected      func(leader string)
}

type LeaderOptions struct {
	LeaseDuration time.Duration
	RetryPeriod   time.Duration
	RenewDeadline time.Duration

	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
}

var _ domain.Component = (*LeaderElection)(nil)

func NewLeaderElection(
	kube *kubeclient.Kubeclient,
	event *event.Event,
	run func(context.Context),
	onElected func(leader string),
	opts LeaderOptions,
) *LeaderElection {
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}

	ko := &LeaderElection{
		name:  types.LeaderLeaseName,
		event: event,
		kube:  kube,
		run:   run,
		opts:  opts,
	}

	ko.election.startedLeading.Store(false)
	ko.election.onElected = onElected
	ko.election.leader = ""

	return ko
}

func (ko *LeaderElection) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create a cancellable context for the leader election
	leaderCtx, cancel := context.WithCancel(ctx)
	ko.cancelFunc = cancel

	go func() {
		leaderelection.RunOrDie(leaderCtx, ko.leaseConfig())
	}()

	ko.started = true
	return nil
}

func (ko *LeaderElection) Started() bool { return ko.started }

func (ko *LeaderElection) Shutdown(ctx context.Context) {
	logger.Info().Msg("🛑 Shutting down leader election...")

	// Cancel the leader election context
	if ko.cancelFunc != nil {
		ko.cancelFunc()
	}

	// Give it a moment to release the lease
	time.Sleep(2 * time.Second)
	logger.Info().Msgf("%s Leader election shut down", term.SuccessMark())
}

func (ko *LeaderElection) Name() string {
	return ko.name
}

func (ko *LeaderElection) kind() string {
	return "Lease"
}

// Helpers
// Lease configuration
func (ko *LeaderElection) leaseConfig() leaderelection.LeaderElectionConfig {
	return leaderelection.LeaderElectionConfig{
		Name:            ko.Name(),
		Lock:            ko.leaseLock(),
		LeaseDuration:   ko.opts.LeaseDuration,
		RenewDeadline:   ko.opts.RenewDeadline,
		RetryPeriod:     ko.opts.RetryPeriod,
		ReleaseOnCancel: true,
		Callbacks:       ko.callbacks(),
	}
}

// Lease lock
func (ko *LeaderElection) leaseLock() *resourcelock.LeaseLock {
	return kubeutil.NewLeaseLock(ko.kube.Clientset(), kubeutil.LeaseOptions{
		Name:        ko.name,
		Namespace:   ko.opts.Namespace,
		Labels:      ko.opts.Labels,
		Annotations: ko.opts.Annotations,
		Recorder:    ko.event.Recorder(),
	})
}

// Build callbacks
func (ko *LeaderElection) callbacks() leaderelection.LeaderCallbacks {
	return leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			if ko.event.Recorder() != nil {
				ko.event.Recorder().Eventf(
					&corev1.ObjectReference{
						Name:      ko.name,
						Namespace: ko.opts.Namespace,
						Kind:      ko.kind(),
					}, corev1.EventTypeNormal, "LeaderElected", "%s became leader", hostname(),
				)
			}

			// Run the actual coordinator
			// With a cancellable context - useful for OnStoppedLeading
			runCtx, cancel := context.WithCancel(ctx)
			ko.runCancel = cancel

			ko.election.leader = hostname()
			if ko.election.onElected != nil {
				ko.election.onElected(ko.election.leader)
			}

			ko.election.startedLeading.Store(true)

			logger.Info().Msgf("%s 🏆 became leader, starting coordinator...", ko.election.leader)

			ko.run(runCtx)
		},
		OnStoppedLeading: func() {
			if ko.event.Recorder() != nil {
				ko.event.Recorder().Eventf(
					&corev1.ObjectReference{
						Name:      ko.name,
						Namespace: ko.opts.Namespace,
						Kind:      ko.kind(),
					}, corev1.EventTypeWarning, "LeaderLost", "%s lost leadership", hostname(),
				)
			}
			if ko.election.startedLeading.Load() {
				// Cancel the run context
				if ko.runCancel != nil {
					ko.runCancel()
					ko.runCancel = nil // reset to always reflect current leadership session only
				}

				logger.Info().Msg("Performing cleanup on the actual leader...")
				ko.election.leader = ""
				ko.election.startedLeading.Store(false)
			} else {
				logger.Info().Msg("No cleanup needed as we never started leading.")
			}

			logger.Info().Msgf("%s 👋 Stopped leading - lease released", hostname())
		},
		OnNewLeader: func(identity string) {
			if ko.event.Recorder() != nil {
				ko.event.Recorder().Eventf(
					&corev1.ObjectReference{
						Name:      ko.name,
						Namespace: ko.opts.Namespace,
						Kind:      ko.kind(),
					}, corev1.EventTypeNormal, "NewLeaderElected", "%s elected as leader", hostname(),
				)
			}
			logger.Info().Msgf("👑 New leader elected: %s", identity)
		},
	}
}

// hostname is this replica's name on the Lease.
func hostname() string { return kubeutil.LeaseIdentity() }

// Leader returns the instance that won the leader election
func (ko *LeaderElection) Leader() string {
	return ko.election.leader
}
