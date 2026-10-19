package webhook

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
	"github.com/intentd/inrun/pkg/logger"
)

const registerTimeout = 30 * time.Second

// register applies the configuration of each webhook the Blueprint declares,
// in the background and best-effort: the housekeeper retries what fails.
func (ws *Server) register() {
	if ws.client == nil {
		return
	}
	bp := ws.bp

	if (bp.HasValidationRules() || bp.HasMutationRules()) && bp.AdmissionRegistry() != nil {
		ws.registerAsync("admission", func(ctx context.Context) error {
			return registration.RegisterAdmissionWebhooks(ctx, ws.client, bp.AdmissionRegistry(), ws.hookReg)
		})
	}

	if gvrs := bp.DeletionProtectionGVRs(); bp.DeletionProtectionActive() && len(gvrs) > 0 {
		ws.registerWithCA(registration.DeletionProtectionConfigName, func(ctx context.Context, caBundle []byte) error {
			return registration.RegisterDeletionProtectionWebhook(ctx, ws.client, gvrs, caBundle, ws.hookReg)
		})
	}

	if bp.NamespaceProtectionActive() {
		gvrs := bp.NamespaceProtectionGVRs()
		policy := bp.NamespaceProtectionFailurePolicy()
		ws.registerWithCA(registration.NamespaceProtectionConfigName, func(ctx context.Context, caBundle []byte) error {
			return registration.RegisterNamespaceProtectionWebhook(ctx, ws.client, gvrs, caBundle, ws.hookReg, policy)
		})
	}

	if bp.IsStrictModeEnabled() {
		ws.registerWithCA(registration.StrictModeConfigName, func(ctx context.Context, caBundle []byte) error {
			return registration.RegisterStrictModeWebhook(ctx, ws.client, caBundle, ws.hookReg)
		})
	}
}

// registerWithCA reads the CA bundle and applies a protection configuration.
func (ws *Server) registerWithCA(config string, apply func(ctx context.Context, caBundle []byte) error) {
	ws.registerAsync(config, func(ctx context.Context) error {
		caBundle, err := registration.ReadCABundle(ws.hookReg.TLSCertFile)
		if err != nil {
			return err
		}
		return apply(ctx, caBundle)
	})
}

// registerAsync runs apply in the background and logs how it went.
func (ws *Server) registerAsync(config string, apply func(ctx context.Context) error) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
		defer cancel()
		if err := apply(ctx); err != nil {
			logger.Error().Err(err).Str("config", config).Msg("webhook registration failed — RBAC may be missing")
			return
		}
		logger.Info().Str("config", config).Msg("webhook registered")
	}()
}

// cleanup removes the configurations the Blueprint asks to clean up on
// shutdown, and the generated TLS Secret with them.
func (ws *Server) cleanup(ctx context.Context) {
	if ws.client == nil {
		return
	}
	bp := ws.bp

	opts := registration.CleanupOptions{
		Mutating:   bp.HasMutationRules() && bp.WebhookCleanupOnShutdown(),
		Validating: bp.HasValidationRules() && bp.WebhookCleanupOnShutdown(),
	}
	if err := registration.UnregisterAdmissionWebhooks(ctx, ws.client, opts); err != nil {
		logger.Error().Err(err).Msg("webhook cleanup error")
	}

	// Strict mode is part of deletion protection and follows its cleanupOnShutdown.
	remove := []struct {
		on   bool
		name string
	}{
		{bp.IsDeletionProtectionEnabled() && bp.DeletionProtectionCleanupOnShutdown(), registration.DeletionProtectionConfigName},
		{bp.IsNamespaceProtectionEnabled() && bp.NamespaceProtectionCleanupOnShutdown(), registration.NamespaceProtectionConfigName},
		{bp.IsStrictModeEnabled() && bp.DeletionProtectionCleanupOnShutdown(), registration.StrictModeConfigName},
	}
	for _, r := range remove {
		if !r.on {
			continue
		}
		if err := registration.RemoveValidatingWebhook(ctx, ws.client, r.name); err != nil {
			logger.Error().Err(err).Str("config", r.name).Msg("webhook cleanup error")
			continue
		}
		logger.Info().Str("config", r.name).Msg("webhook removed")
	}

	cleanupTLS := bp.WebhookCleanupOnShutdown() ||
		bp.NamespaceProtectionCleanupOnShutdown() ||
		bp.DeletionProtectionCleanupOnShutdown()
	if ws.certMgr != nil && cleanupTLS {
		ns := ws.cfg.Cluster().Namespace()
		if err := ws.certMgr.DeleteCertificateAndSecret(ctx, ns, config.DefaultInternalTLSName()); err != nil {
			logger.Error().Err(err).Msg("tls secret cleanup error")
		} else {
			logger.Info().Str("namespace", ns).Msg("tls secret removed on shutdown")
		}
	}
}
