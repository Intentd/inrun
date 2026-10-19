package housekeeper

import (
	"context"
	"sync/atomic"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/util/kube"
)

var _ domain.Component = (*Housekeeper)(nil)

const (
	highTimeout = 30 * time.Second
	lowTimeout  = 10 * time.Second
)

// Metric labels for what the housekeeper syncs.
const (
	labelHousekeeper         = "housekeeper"
	labelValidation          = "validation"
	labelMutation            = "mutation"
	labelDeletionProtection  = "deletion-protection"
	labelNamespaceProtection = "namespace-protection"
	labelStrictMode          = "strict-mode-protection"
	labelTLSSecret           = "tls-secret"
	labelTLSRotation         = "tls-secret-rotation"
	labelTokenSecret         = "token-secret"
	labelNamespaceLabels     = "namespace-labels"
	labelConversionCABundle  = "conversion-crd-cabundle"
)

// Housekeeper keeps the gateway's infrastructure in sync. Create it with New,
// give it what the gateway generated with the setters, and start it as a
// domain.Component after the webhook server.
type Housekeeper struct {
	client    kubernetes.Interface
	bp        *blueprint.Blueprint
	cfg       *config.Config
	opts      registration.Options
	admission blueprint.AdmissionRegistry

	// The generated TLS bundle and where its Secret lives. Nil when the
	// certificate is configured rather than generated.
	cert          *certBundle
	certName      string
	certNamespace string

	patchConversionCRD ConversionCRDPatchFn
	crdWatcher         CRDWatcher
	reloadTokens       func(ctx context.Context) error

	stats *health.WebhookStats

	started atomic.Bool
	cancel  context.CancelFunc
}

// New creates a Housekeeper for the Blueprint. It does no I/O.
func New(client kubernetes.Interface, bp *blueprint.Blueprint, cfg *config.Config) *Housekeeper {
	opts := registration.NewOptions(bp, cfg)
	opts.Caller = registration.CallerHousekeeper
	return &Housekeeper{
		client:    client,
		bp:        bp,
		cfg:       cfg,
		opts:      opts,
		admission: bp.AdmissionRegistry(),
		stats:     health.NewWebhookStats(),
	}
}

// SetTokenReloader gives the housekeeper the Gateway API's token reload, which
// recreates missing token Secrets and reloads the tokens in memory.
func (h *Housekeeper) SetTokenReloader(fn func(ctx context.Context) error) {
	h.reloadTokens = fn
}

// Stats returns how many syncs ran and failed.
func (h *Housekeeper) Stats() *health.WebhookStats { return h.stats }

// Start begins syncing in the background. Outside a pod, or when the
// Blueprint turns the housekeeper off, it does nothing.
func (h *Housekeeper) Start(ctx context.Context) error {
	h.started.Store(true)
	if !kube.IsRunningInPod() || !h.bp.IsHousekeeperEnabled() {
		return nil
	}
	ctx, h.cancel = context.WithCancel(ctx)
	h.run(ctx)
	return nil
}

// Shutdown stops syncing. It removes nothing; the webhook server's shutdown
// does the cleanup the Blueprint asks for.
func (h *Housekeeper) Shutdown(context.Context) {
	if h.cancel != nil {
		h.cancel()
	}
}

// Name returns the component name.
func (h *Housekeeper) Name() string { return "housekeeper" }

// Started reports whether Start has been called.
func (h *Housekeeper) Started() bool { return h.started.Load() }

// run starts the watches and the sync loop.
func (h *Housekeeper) run(ctx context.Context) {
	if h.client == nil {
		logger.Debug().Msg("housekeeper disabled: no kube client")
		return
	}
	if !h.hasWork() {
		logger.Debug().Msg("housekeeper disabled: no admission, protection, conversion, or serve declared")
		return
	}

	// Capacity 1: a burst of events leaves one sync pending.
	trigger := make(chan struct{}, 1)
	h.startWatches(ctx, trigger)

	go func() {
		ticker := time.NewTicker(h.bp.HousekeeperSyncInterval())
		defer ticker.Stop()

		h.syncAll()
		for {
			select {
			case <-ctx.Done():
				logger.Debug().Msg("housekeeper stopping")
				return
			case <-trigger:
				logger.Warn().Msg("housekeeper: change detected, syncing")
				h.syncAll()
			case <-ticker.C:
				h.syncAll()
			}
		}
	}()

	logger.Info().Msg("housekeeper started")
}

// hasWork reports whether the Blueprint declares anything to keep in sync.
func (h *Housekeeper) hasWork() bool {
	bp := h.bp
	return bp.UsesValidatingWebhooks() || bp.HasMutationRules() ||
		bp.IsConversionEnabled() || bp.HasServeEnabled()
}

// syncAll syncs everything once. The TLS Secret goes first, since every
// webhook configuration trusts it.
func (h *Housekeeper) syncAll() {
	h.stats.RecordReconciled()
	metrics.RecordWebhookReconciled(labelHousekeeper)

	h.syncCertSecret()
	h.syncAdmissionWebhooks()
	h.syncDeletionProtectionWebhook()
	h.syncNamespaceProtectionWebhook()
	h.syncStrictModeWebhook()
	h.syncNamespaceLabels()
	h.syncConversionCRDs()
	h.syncTokenSecrets()
}

// failed records a failed sync of the webhook configurations behind labels.
func (h *Housekeeper) failed(labels ...string) {
	h.stats.RecordFailure()
	for _, l := range labels {
		metrics.RecordWebhookReconciliationFailure(l)
	}
}
