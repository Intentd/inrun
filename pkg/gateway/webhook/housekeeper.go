// webhook/housekeeper.go — self-healing maintenance of Inrun's admission surface.
//
// The housekeeper runs in the background and ensures that the webhook
// configurations Inrun owns exist exactly when the Blueprint declares them —
// and disappear when they do not.
//
// Declarative Webhook Lifecycle
//
//	security.admission.enabled: true
//	    → create/update the validating and mutating webhook configurations
//
//	security.deletionProtection.enabled: true
//	    → create/update the deletion-protection webhook
//
//	(inverse: when disabled, the housekeeper removes the corresponding webhook)
//
// # Event-Driven Reconciliation
//
// The housekeeper uses a Kubernetes Watch on ValidatingWebhookConfiguration and
// MutatingWebhookConfiguration objects as the fast path. A DELETED event
// triggers an immediate reconcile — the window during which a webhook is absent
// is bounded only by the API server round-trip, not by a poll interval.
// MODIFIED events are intentionally ignored: every reconcile Update fires one,
// and reacting to it would create a tight loop. Content drift is caught by the
// safety ticker instead.
//
// A safety ticker (HOUSEKEEPER_SYNC_INTERVAL, default 30 s) runs in
// parallel as a backstop for drift that Watch silently misses: partial mutations,
// silent stream drops on some managed clusters, and token expiry.
//
// A buffered trigger channel (capacity 1) coalesces bursts — any number of rapid
// DELETE/MODIFY events produce exactly one reconcile call.
//
// # Cleanup Semantics
//
// Webhook configurations are not removed on pod restart by default. Cleanup is
// opt-in via security.admission.cleanupOnShutdown and
// security.deletionProtection.cleanupOnShutdown. This avoids race conditions
// during rollout where a new leader sees an existing webhook while the old leader
// is still shutting down.
//
// The housekeeper returns only fatal initialization errors. Reconciliation itself
// is continuous and non-blocking.
package webhook

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

const (
	highTimeout = 30 * time.Second
	lowTimeout  = 10 * time.Second

	// watchRetryDelay is the pause before re-establishing a dropped Watch stream.
	watchRetryDelay = 5 * time.Second

	// inrunWebhookLabelSelector narrows the Watch to Inrun-owned webhook
	// configurations — unrelated webhooks in the same cluster are ignored.
	inrunWebhookLabelSelector = "app.kubernetes.io/tag=inrun-internal"
)

// housekeeper continuously reconciles all admission webhook configurations
// declared by the Blueprint. Runs background goroutines until ctx is cancelled.
// Called once on leader startup. Returns only fatal initialization errors.
func (ws *WebhookServer) housekeeper(ctx context.Context) error {
	if ws.kubeClient == nil || ws.blueprint == nil {
		logger.Debug().Msg("housekeeper disabled: kube client or blueprint not set")
		return nil
	}

	bp := ws.blueprint
	hasAdmission := bp.HasValidationRules() || bp.HasMutationRules()
	hasDeletionProtection := bp.IsDeletionProtectionEnabled() && bp.DeletionProtectionGVRs() != nil
	hasNamespaceProtection := bp.IsNamespaceProtectionEnabled() && len(bp.NamespaceProtectionGVRs()) > 0
	hasStrictMode := bp.IsStrictModeEnabled()
	hasConversion := ws.convEnabled
	hasServe := bp.HasServeEnabled()

	if !hasAdmission && !hasDeletionProtection && !hasNamespaceProtection && !hasStrictMode && !hasConversion && !hasServe {
		logger.Debug().Msg("housekeeper disabled: no admission, protection, conversion, or serve declared")
		return nil
	}

	// Buffered capacity 1: bursts of Watch events collapse into one reconcile.
	trigger := make(chan struct{}, 1)

	// The TLS Secret is the dependency for everything else — watch it first.
	// If it is deleted (e.g. by a concurrent pod with cleanupOnShutdown during rollout),
	// restore it immediately from the in-memory bundle before webhook configs break.
	if ws.certSecretData != nil {
		go ws.watchCertSecret(ctx, trigger)
	}

	// Validation, deletion‑protection, namespace‑protection, and strict‑mode all use
	// ValidatingWebhookConfiguration, so they share the same watcher.
	// Mutation rules use a separate MutatingWebhookConfiguration watcher.
	if bp.HasValidationRules() || hasDeletionProtection || hasNamespaceProtection || hasStrictMode {
		go ws.watchValidatingWebhooks(ctx, trigger)
	}

	if bp.HasMutationRules() {
		go ws.watchMutatingWebhooks(ctx, trigger)
	}

	// CRD conversion caBundle watcher — one goroutine per conversion CRD.
	// Triggers immediate reconcile on any MODIFIED event so a stripped caBundle
	// is restored within one API round-trip, not at the next safety-ticker interval.
	if hasConversion {
		ws.watchConversionCRDs(ctx, trigger)
	}

	go func() {
		safetyTicker := time.NewTicker(bp.HousekeeperSyncInterval())
		defer safetyTicker.Stop()

		// Reconcile immediately on startup — don't wait for the first tick or event.
		ws.reconcileAll()

		for {
			select {
			case <-ctx.Done():
				logger.Debug().Msg("housekeeper stopping")
				return
			case <-trigger:
				logger.Warn().Msg("housekeeper: change detected — reconciling")
				ws.reconcileAll()
			case <-safetyTicker.C:
				ws.reconcileAll()
			}
		}
	}()

	logger.Info().Msg("housekeeper started")
	return nil
}

// reconcileAll records metrics and drives all reconcile functions.
// Order matters: TLS Secret first (all webhook registrations depend on it),
// then webhook configurations, then infrastructure state.
func (ws *WebhookServer) reconcileAll() {
	if ws.webhookStats != nil {
		ws.webhookStats.RecordReconciled()
	}
	metrics.RecordWebhookReconciled("housekeeper")

	ws.reconcileCertSecret()
	ws.reconcileAdmissionWebhooks()
	ws.reconcileDeletionProtectionWebhook()
	ws.reconcileNamespaceProtectionWebhook()
	ws.reconcileStrictModeWebhook()

	// Infrastructure state — applied once at startup by ensureSecurity,
	// kept correct here throughout the deployment lifecycle.
	ws.reconcileNamespaceLabels()
	ws.reconcileCRDConversionWebhooks()

	ws.reconcileTokenSecrets()
}

// reconcileCertSecret ensures the TLS Secret exists in the cluster and,
// when auto-rotation is enabled, pre-emptively rotates the certificate before expiry.
func (ws *WebhookServer) reconcileCertSecret() {
	if ws.certSecretData == nil || ws.kubeClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	existing, err := ws.kubeClient.CoreV1().Secrets(ws.certSecretNamespace).
		Get(ctx, ws.certSecretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error().Err(err).Str("secret", ws.certSecretName).
				Msg("housekeeper: failed to check TLS secret")
			metrics.RecordWebhookReconciliationFailure("tls-secret")
			return
		}
		ws.restoreCertSecret(ctx)
		return
	}

	// Secret exists — check if pre-emptive rotation is needed.
	if ws.blueprint != nil && ws.blueprint.CertAutoRotate() {
		ws.maybeRotateCert(ctx, existing)
	}
}

// restoreCertSecret re-creates the TLS Secret from the in-memory bundle.
// Called when the Secret has been deleted (e.g. by a concurrent pod with
// cleanupOnShutdown during a rolling restart).
func (ws *WebhookServer) restoreCertSecret(ctx context.Context) {
	logger.Warn().
		Str("secret", ws.certSecretName).
		Str("namespace", ws.certSecretNamespace).
		Msg("housekeeper: TLS secret missing — restoring from in-memory bundle")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.certSecretName,
			Namespace: ws.certSecretNamespace,
			Labels:    labels.WithDeletionProtection(labels.InrunResourceLabels()),
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": ws.certSecretData.certPEM,
			"tls.key": ws.certSecretData.keyPEM,
			"ca.crt":  ws.certSecretData.caPEM,
		},
	}

	if _, err := ws.kubeClient.CoreV1().Secrets(ws.certSecretNamespace).
		Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			logger.Error().Err(err).Str("secret", ws.certSecretName).
				Msg("housekeeper: failed to restore TLS secret")
			metrics.RecordWebhookReconciliationFailure("tls-secret")
		}
		return
	}

	logger.Info().Str("secret", ws.certSecretName).
		Msg("housekeeper: TLS secret restored")
	metrics.RecordWebhookReconciled("tls-secret")
}

// maybeRotateCert checks whether the stored TLS certificate is within the
// rotation threshold and, if so, generates a new bundle and updates the Secret.
// The running HTTPS server continues serving the old certificate (still valid
// for the full threshold window). The new certificate takes effect on the next
// gateway restart — this is pre-emptive, not live rotation.
func (ws *WebhookServer) maybeRotateCert(ctx context.Context, existing *corev1.Secret) {
	if ws.config == nil {
		return
	}

	threshold := ws.blueprint.CertRotationThreshold()

	// Parse expiry from the cert stored in the Secret (ground truth for next restart).
	certData := existing.Data["tls.crt"]
	if len(certData) == 0 {
		return
	}
	block, _ := pem.Decode(certData)
	if block == nil {
		return
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return
	}

	if time.Now().Add(threshold).Before(cert.NotAfter) {
		return // cert is fine, not yet within rotation window
	}

	daysLeft := time.Until(cert.NotAfter).Hours() / 24
	logger.Warn().
		Str("secret", ws.certSecretName).
		Float64("days_until_expiry", daysLeft).
		Msg("housekeeper: TLS cert within rotation window — pre-emptive rotation")

	svcName := ws.blueprint.GatewayServiceName()
	ns := ws.certSecretNamespace

	newBundle, err := certmanager.GenerateClusterBundle(svcName, ns, certmanager.BundleOpts{ValidFor: ws.blueprint.CertValidForStr()})
	if err != nil {
		logger.Error().Err(err).Msg("housekeeper: cert rotation — failed to generate new bundle")
		metrics.RecordWebhookReconciliationFailure("tls-secret-rotation")
		return
	}

	// Re-fetch immediately before Update to get the latest ResourceVersion.
	// This avoids a UID/ResourceVersion conflict when restoreCertSecret ran
	// concurrently (e.g. cleanupOnShutdown deleted the Secret mid-reconcile).
	fresh, err := ws.kubeClient.CoreV1().Secrets(ws.certSecretNamespace).
		Get(ctx, ws.certSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Secret was deleted between our Get and now — let the next reconcile handle it.
			logger.Warn().Msg("housekeeper: cert rotation — secret vanished before update, skipping")
		} else {
			logger.Error().Err(err).Msg("housekeeper: cert rotation — re-fetch before update failed")
			metrics.RecordWebhookReconciliationFailure("tls-secret-rotation")
		}
		return
	}

	fresh.Data["tls.crt"] = newBundle.CertPEM
	fresh.Data["tls.key"] = newBundle.KeyPEM
	fresh.Data["ca.crt"] = newBundle.CACertPEM
	if fresh.Annotations == nil {
		fresh.Annotations = map[string]string{}
	}
	fresh.Annotations["inrun.dev/rotated-at"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := ws.kubeClient.CoreV1().Secrets(ws.certSecretNamespace).
		Update(ctx, fresh, metav1.UpdateOptions{}); err != nil {
		logger.Error().Err(err).Msg("housekeeper: cert rotation — failed to update secret")
		metrics.RecordWebhookReconciliationFailure("tls-secret-rotation")
		return
	}

	// Keep in-memory bundle in sync — so if the Secret is deleted after rotation,
	// the housekeeper restores the rotated cert, not the original one.
	ws.certSecretData = &certSecretBundle{
		certPEM: newBundle.CertPEM,
		keyPEM:  newBundle.KeyPEM,
		caPEM:   newBundle.CACertPEM,
	}

	logger.Info().
		Str("secret", ws.certSecretName).
		Msg("housekeeper: TLS cert rotated — new cert takes effect on next gateway restart")
	metrics.RecordWebhookReconciled("tls-secret-rotation")

}

// watchCertSecret watches the TLS Secret for DELETED events and triggers
// an immediate reconcile so the Secret is restored before the next pod
// restart picks up a mismatched certificate.
func (ws *WebhookServer) watchCertSecret(ctx context.Context, trigger chan<- struct{}) {
	for {
		watcher, err := ws.kubeClient.CoreV1().Secrets(ws.certSecretNamespace).
			Watch(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + ws.certSecretName,
			})
		if err != nil {
			logger.Warn().Err(err).Msg("housekeeper watch (tls-secret): failed to start, retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(watchRetryDelay):
				continue
			}
		}

		ws.drainWatchEvents(ctx, watcher, trigger, "tls-secret")
		watcher.Stop()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// watchValidatingWebhooks watches ValidatingWebhookConfiguration objects owned
// by Inrun and sends to trigger on DELETED or MODIFIED events.
// Reconnects automatically when the stream expires or drops.
func (ws *WebhookServer) watchValidatingWebhooks(ctx context.Context, trigger chan<- struct{}) {
	for {
		watcher, err := ws.kubeClient.AdmissionregistrationV1().
			ValidatingWebhookConfigurations().
			Watch(ctx, metav1.ListOptions{
				LabelSelector: inrunWebhookLabelSelector,
			})
		if err != nil {
			logger.Warn().Err(err).Msg("housekeeper watch (validating): failed to start, retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(watchRetryDelay):
				continue
			}
		}

		ws.drainWatchEvents(ctx, watcher, trigger, "validating")
		watcher.Stop()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// watchMutatingWebhooks watches MutatingWebhookConfiguration objects owned
// by Inrun and sends to trigger on DELETED or MODIFIED events.
// Reconnects automatically when the stream expires or drops.
func (ws *WebhookServer) watchMutatingWebhooks(ctx context.Context, trigger chan<- struct{}) {
	for {
		watcher, err := ws.kubeClient.AdmissionregistrationV1().
			MutatingWebhookConfigurations().
			Watch(ctx, metav1.ListOptions{
				LabelSelector: inrunWebhookLabelSelector,
			})
		if err != nil {
			logger.Warn().Err(err).Msg("housekeeper watch (mutating): failed to start, retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(watchRetryDelay):
				continue
			}
		}

		ws.drainWatchEvents(ctx, watcher, trigger, "mutating")
		watcher.Stop()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// drainWatchEvents reads events from a watcher until the channel closes or ctx
// is cancelled. Only DELETED events signal the trigger channel.
//
// MODIFIED is intentionally ignored. Every reconcile calls Update on the
// webhook configuration, which always fires a MODIFIED event — reacting to it
// would immediately re-trigger the reconcile that just finished, creating a
// tight reconcile→update→MODIFIED→reconcile loop. Content drift from external
// modifications is caught within one safety-ticker interval instead.
func (ws *WebhookServer) drainWatchEvents(ctx context.Context, w watch.Interface, trigger chan<- struct{}, kind string) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.ResultChan():
			if !ok {
				// Channel closed: Watch stream expired or network error. Reconnect.
				return
			}
			if event.Type == watch.Deleted {
				logger.Warn().Str("kind", kind).Msg("housekeeper: configuration deleted — triggering reconcile")
				select {
				case trigger <- struct{}{}:
				default: // already pending
				}
			}
		}
	}
}

func (ws *WebhookServer) reconcileAdmissionWebhooks() {
	bp := ws.blueprint

	if !ws.webhooksEnabled || ws.admissionRegistry == nil {
		cleanupOpts := WebhookCleanupOptions{}
		if bp.HasMutationRules() {
			cleanupOpts.mutating = true
		}
		if bp.HasValidationRules() {
			cleanupOpts.validating = true
		}
		if (cleanupOpts.mutating || cleanupOpts.validating) && ws.kubeClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
			defer cancel()
			if err := UnregisterAdmissionWebhooks(ctx, ws.kubeClient, cleanupOpts); err != nil {
				logger.Error().Err(err).Msg("housekeeper: admission cleanup failed")
				if ws.webhookStats != nil {
					ws.webhookStats.RecordFailure()
				}
				if cleanupOpts.validating {
					metrics.RecordWebhookReconciliationFailure(validation)
				}
				if cleanupOpts.mutating {
					metrics.RecordWebhookReconciliationFailure(mutation)
				}
			}
		}
		return
	}

	if !bp.HasValidationRules() && !bp.HasMutationRules() {
		return
	}

	if ws.kubeClient != nil && ws.admissionRegistry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
		defer cancel()

		// logger information
		ws.hookReg.Caller = housekeeper

		if err := RegisterAdmissionWebhooks(ctx, ws.kubeClient, ws.admissionRegistry, ws.hookReg); err != nil {
			logger.Error().Err(err).Msgf("%s: admission registration failed", housekeeper)
			if ws.webhookStats != nil {
				ws.webhookStats.RecordFailure()
			}
			if bp.HasValidationRules() {
				metrics.RecordWebhookReconciliationFailure(validation)
			}
			if bp.HasMutationRules() {
				metrics.RecordWebhookReconciliationFailure(mutation)
			}
		}
	}
}

func (ws *WebhookServer) reconcileDeletionProtectionWebhook() {
	bp := ws.blueprint

	enabled := bp.IsDeletionProtectionEnabled() && bp.DeletionProtectionGVRs() != nil
	if !enabled {
		if ws.kubeClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
			defer cancel()
			if err := cleanupValidatingWebhook(ctx, ws.kubeClient, deletionProtectionWebhookConfigName); err != nil {
				logger.Debug().Err(err).Msgf("%s: deletion protection cleanup skipped or failed", housekeeper)
				if ws.webhookStats != nil {
					ws.webhookStats.RecordFailure()
				}
				metrics.RecordWebhookReconciliationFailure(deletionProtection)
			}
		}
		return
	}

	if ws.kubeClient == nil {
		return
	}

	dpGVRs := bp.DeletionProtectionGVRs()
	if len(dpGVRs) == 0 {
		return
	}

	caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
	if err != nil {
		logger.Error().Err(err).Msgf("%s: cannot read CA bundle for deletion protection", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(deletionProtection)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	if err := registerDeletionProtectionWebhook(ctx, ws.kubeClient, dpGVRs, caBundle, ws.hookReg); err != nil {
		logger.Error().Err(err).Msgf("%s: deletion protection registration failed", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(deletionProtection)
	}
}

func (ws *WebhookServer) reconcileNamespaceProtectionWebhook() {
	bp := ws.blueprint

	enabled := bp.IsNamespaceProtectionEnabled() && len(bp.NamespaceProtectionGVRs()) > 0
	if !enabled {
		if ws.kubeClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
			defer cancel()
			if err := cleanupValidatingWebhook(ctx, ws.kubeClient, namespaceProtectionWebhookConfigName); err != nil {
				logger.Debug().Err(err).Msgf("%s: namespace protection cleanup skipped or failed", housekeeper)
				if ws.webhookStats != nil {
					ws.webhookStats.RecordFailure()
				}
				metrics.RecordWebhookReconciliationFailure(namespaceProtection)
			}
		}
		return
	}

	if ws.kubeClient == nil {
		return
	}

	npGVRs := bp.NamespaceProtectionGVRs()
	if len(npGVRs) == 0 {
		return
	}

	caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
	if err != nil {
		logger.Error().Err(err).Msgf("%s: cannot read CA bundle for namespace protection", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(namespaceProtection)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	svcName := bp.GatewayServiceName()
	failurePolicy := bp.NamespaceProtectionFailurePolicy()
	if err := registerNamespaceProtectionWebhook(ctx, ws.kubeClient, npGVRs, caBundle, ws.hookReg, svcName, failurePolicy); err != nil {
		logger.Error().Err(err).Msgf("%s: namespace protection registration failed", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(namespaceProtection)
	}
}

// reconcileTokenSecrets checks that every Gateway API secretRef token secret exists.
// If any are missing, the tokenReloader callback is invoked to recreate them and
// refresh the in-memory token set. Detection relies on the safety ticker — token
// secret deletion does not break in-flight requests (the TokenSet is in memory),
// so the 30 s window is acceptable.
func (ws *WebhookServer) reconcileTokenSecrets() {
	if ws.kubeClient == nil || ws.blueprint == nil || ws.config == nil {
		return
	}
	bp := ws.blueprint
	if !bp.HasGatewayAPISecretRefs() {
		return
	}

	ownNS := ws.config.Cluster().Namespace()

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	anyMissing := false
	for _, t := range bp.Gateway.API.Auth.Tokens {
		if t.SecretRef == nil {
			continue
		}
		ns := t.SecretRef.Namespace
		if ns == "" {
			ns = ownNS
		}
		_, err := ws.kubeClient.CoreV1().Secrets(ns).Get(ctx, t.SecretRef.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logger.Warn().
					Str("secret", t.SecretRef.Name).
					Str("namespace", ns).
					Str("token", t.Name).
					Msg("housekeeper: gateway API token secret missing — will reload")
				anyMissing = true
			} else {
				logger.Error().Err(err).
					Str("secret", t.SecretRef.Name).
					Str("namespace", ns).
					Msg("housekeeper: failed to check gateway API token secret")
				metrics.RecordWebhookReconciliationFailure("token-secret")
			}
		}
	}

	if anyMissing && ws.tokenReloader != nil {
		if err := ws.tokenReloader(ctx); err != nil {
			logger.Error().Err(err).Msg("housekeeper: gateway API token reload failed")
			metrics.RecordWebhookReconciliationFailure("token-secret")
		} else {
			logger.Info().Msg("housekeeper: gateway API tokens reloaded")
			metrics.RecordWebhookReconciled("token-secret")
		}
	}
}

func (ws *WebhookServer) reconcileStrictModeWebhook() {
	bp := ws.blueprint

	if !bp.IsStrictModeEnabled() {
		if ws.kubeClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
			defer cancel()
			if err := cleanupValidatingWebhook(ctx, ws.kubeClient, strictModeProtectionWebhookConfigName); err != nil {
				logger.Debug().Err(err).Msgf("%s: strict mode cleanup skipped or failed", housekeeper)
				if ws.webhookStats != nil {
					ws.webhookStats.RecordFailure()
				}
				metrics.RecordWebhookReconciliationFailure(strictModeProtection)
			}
		}
		return
	}

	if ws.kubeClient == nil {
		return
	}

	caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
	if err != nil {
		logger.Error().Err(err).Msgf("%s: cannot read CA bundle for strict mode protection", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(strictModeProtection)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	if err := registerStrictModeProtectionWebhook(ctx, ws.kubeClient, caBundle, ws.hookReg); err != nil {
		logger.Error().Err(err).Msgf("%s: strict mode protection registration failed", housekeeper)
		if ws.webhookStats != nil {
			ws.webhookStats.RecordFailure()
		}
		metrics.RecordWebhookReconciliationFailure(strictModeProtection)
	}
}
