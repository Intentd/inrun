// Package event records Kubernetes events for reconcilers and other
// components without depending on client-go's recorder directly. Discard is
// a no-op recorder for tests and simulation.
package event

import (
	"context"
	"sync"

	"errors"
	"github.com/intentd/inrun/domain"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// Recorder is the interface for emitting Kubernetes events.
// *Event implements this for real clusters; Discard() returns a silent no-op.
type Recorder interface {
	Eventf(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{})
}

// Discard returns a Recorder that silently drops all events.
// Passed to constructors by inrun simulate so ev is always non-nil.
func Discard() Recorder { return discardEvent{} }

type discardEvent struct{}

func (discardEvent) Eventf(runtime.Object, string, string, string, ...interface{}) {}

type Event struct {
	name        string
	kube        *kubeclient.Kubeclient
	scheme      *runtime.Scheme
	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder
	component   string
	stopped     bool           // Track state
	wg          sync.WaitGroup // Track in-flight events
	mu          sync.Mutex     // Protect shutdown
	started     bool
}

var _ domain.Component = (*Event)(nil)

func NewEvent(kube *kubeclient.Kubeclient) *Event {
	if kube.Scheme() == nil {
		utils.Exit(errors.New("scheme cannot be nil"))
	}

	return &Event{
		name:      "event handler",
		component: "inrun runtime",
		kube:      kube,
		scheme:    kube.Scheme(),
	}
}

func (e *Event) Start(ctx context.Context) error {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create event broadcaster
	e.broadcaster = record.NewBroadcaster(record.WithContext(ctx))
	e.broadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: e.kube.Clientset().CoreV1().Events(""),
		})

	// Create event recorder
	e.recorder = e.broadcaster.NewRecorder(
		e.scheme,
		corev1.EventSource{
			Component: e.component,
		})

	e.started = true
	return nil
}

// Eventf - track in-flight events
func (e *Event) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.wg.Add(1) // Track this event
	e.mu.Unlock()

	// Record in goroutine so we don't block
	go func() {
		defer e.wg.Done()
		if e.recorder != nil {
			e.recorder.Eventf(object, eventtype, reason, messageFmt, args...)
		}
	}()
}

func (e *Event) Shutdown(ctx context.Context) {
	logger.Info().Msgf("shutting down %s...", e.name)

	e.mu.Lock()
	e.stopped = true
	e.mu.Unlock()

	// Wait for in-flight events with timeout
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info().Msg("all events flushed")
	case <-ctx.Done():
		logger.Warn().Msg("timeout waiting for events to flush")
	}

	if e.broadcaster != nil {
		e.broadcaster.Shutdown()
	}
}

// Healthy mark on startup
func (e *Event) Started() bool { return e.started }

func (e *Event) Name() string {
	return e.name
}

func (e *Event) Broadcaster() record.EventBroadcaster {
	return e.broadcaster
}

func (e *Event) Recorder() record.EventRecorder {
	return e.recorder
}
