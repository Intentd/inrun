package housekeeper

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/intentd/inrun/pkg/logger"
)

const (
	// watchRetryDelay is the pause before retrying a watch that failed to start.
	watchRetryDelay = 5 * time.Second

	// inrunWebhookSelector limits the webhook watches to Inrun's own configurations.
	inrunWebhookSelector = "app.kubernetes.io/tag=inrun-internal"
)

// startWatches starts a watch for each kind of resource the Blueprint needs.
func (h *Housekeeper) startWatches(ctx context.Context, trigger chan<- struct{}) {
	// The TLS Secret: a pod shutting down with cleanupOnShutdown may delete
	// it during a rollout, and every webhook configuration depends on it.
	if h.cert != nil {
		go watchLoop(ctx, trigger, "tls-secret", watch.Deleted, func(ctx context.Context) (watch.Interface, error) {
			return h.client.CoreV1().Secrets(h.certNamespace).Watch(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + h.certName,
			})
		})
	}

	if h.bp.UsesValidatingWebhooks() {
		go watchLoop(ctx, trigger, "validating", watch.Deleted, func(ctx context.Context) (watch.Interface, error) {
			return h.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Watch(ctx, metav1.ListOptions{
				LabelSelector: inrunWebhookSelector,
			})
		})
	}

	if h.bp.HasMutationRules() {
		go watchLoop(ctx, trigger, "mutating", watch.Deleted, func(ctx context.Context) (watch.Interface, error) {
			return h.client.AdmissionregistrationV1().MutatingWebhookConfigurations().Watch(ctx, metav1.ListOptions{
				LabelSelector: inrunWebhookSelector,
			})
		})
	}

	h.watchConversionCRDs(ctx, trigger)
}

// watchLoop keeps a watch open until ctx is done, reopening it when the
// stream ends, and triggers a sync on every event of type on.
func watchLoop(ctx context.Context, trigger chan<- struct{}, kind string, on watch.EventType, open func(context.Context) (watch.Interface, error)) {
	for {
		w, err := open(ctx)
		if err != nil {
			logger.Warn().Err(err).Str("kind", kind).Msg("housekeeper: watch failed to start, retrying")
			if !sleep(ctx, watchRetryDelay) {
				return
			}
			continue
		}

		forward(ctx, w, trigger, kind, on)
		w.Stop()

		if !sleep(ctx, time.Second) {
			return
		}
	}
}

// forward reads events until the stream ends or ctx is done, and triggers a
// sync on each event of type on.
func forward(ctx context.Context, w watch.Interface, trigger chan<- struct{}, kind string, on watch.EventType) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.ResultChan():
			if !ok {
				return // stream expired or dropped
			}
			if event.Type != on {
				continue
			}
			logger.Warn().Str("kind", kind).Str("event", string(event.Type)).Msg("housekeeper: triggering sync")
			select {
			case trigger <- struct{}{}:
			default: // a sync is already pending
			}
		}
	}
}

// sleep waits for d and reports false if ctx was done first.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
