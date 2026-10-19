package housekeeper

import (
	"context"

	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
	"github.com/intentd/inrun/pkg/logger"
)

// syncAdmissionWebhooks keeps the validating and mutating configurations in
// line with the admission rules, and removes them when admission is off.
func (h *Housekeeper) syncAdmissionWebhooks() {
	bp := h.bp
	validating, mutating := bp.HasValidationRules(), bp.HasMutationRules()
	if !validating && !mutating {
		return
	}

	failed := func() {
		var labels []string
		if validating {
			labels = append(labels, labelValidation)
		}
		if mutating {
			labels = append(labels, labelMutation)
		}
		h.failed(labels...)
	}

	if !bp.IsAdmissionEnabled() || h.admission == nil {
		ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
		defer cancel()
		opts := registration.CleanupOptions{Validating: validating, Mutating: mutating}
		if err := registration.UnregisterAdmissionWebhooks(ctx, h.client, opts); err != nil {
			logger.Error().Err(err).Msg("housekeeper: admission cleanup failed")
			failed()
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()
	if err := registration.RegisterAdmissionWebhooks(ctx, h.client, h.admission, h.opts); err != nil {
		logger.Error().Err(err).Msg("housekeeper: admission registration failed")
		failed()
	}
}

// syncDeletionProtectionWebhook keeps the deletion-protection configuration,
// and removes it when deletion protection is off.
func (h *Housekeeper) syncDeletionProtectionWebhook() {
	if !h.bp.DeletionProtectionActive() {
		h.remove(registration.DeletionProtectionConfigName, labelDeletionProtection)
		return
	}
	gvrs := h.bp.DeletionProtectionGVRs()
	if len(gvrs) == 0 {
		return
	}
	h.register(labelDeletionProtection, func(ctx context.Context, caBundle []byte) error {
		return registration.RegisterDeletionProtectionWebhook(ctx, h.client, gvrs, caBundle, h.opts)
	})
}

// syncNamespaceProtectionWebhook keeps the namespace-protection configuration,
// and removes it when no CRD restricts its namespaces.
func (h *Housekeeper) syncNamespaceProtectionWebhook() {
	if !h.bp.NamespaceProtectionActive() {
		h.remove(registration.NamespaceProtectionConfigName, labelNamespaceProtection)
		return
	}
	gvrs := h.bp.NamespaceProtectionGVRs()
	policy := h.bp.NamespaceProtectionFailurePolicy()
	h.register(labelNamespaceProtection, func(ctx context.Context, caBundle []byte) error {
		return registration.RegisterNamespaceProtectionWebhook(ctx, h.client, gvrs, caBundle, h.opts, policy)
	})
}

// syncStrictModeWebhook keeps the strict-mode configuration, and removes it
// when strict mode is off.
func (h *Housekeeper) syncStrictModeWebhook() {
	if !h.bp.IsStrictModeEnabled() {
		h.remove(registration.StrictModeConfigName, labelStrictMode)
		return
	}
	h.register(labelStrictMode, func(ctx context.Context, caBundle []byte) error {
		return registration.RegisterStrictModeWebhook(ctx, h.client, caBundle, h.opts)
	})
}

// register reads the CA bundle and applies a protection configuration.
func (h *Housekeeper) register(label string, apply func(ctx context.Context, caBundle []byte) error) {
	caBundle, err := registration.ReadCABundle(h.opts.TLSCertFile)
	if err != nil {
		logger.Error().Err(err).Str("webhook", label).Msg("housekeeper: cannot read CA bundle")
		h.failed(label)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()
	if err := apply(ctx, caBundle); err != nil {
		logger.Error().Err(err).Str("webhook", label).Msg("housekeeper: registration failed")
		h.failed(label)
	}
}

// remove deletes a protection configuration the Blueprint no longer declares.
func (h *Housekeeper) remove(name, label string) {
	ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
	defer cancel()
	if err := registration.RemoveValidatingWebhook(ctx, h.client, name); err != nil {
		logger.Debug().Err(err).Str("webhook", label).Msg("housekeeper: cleanup skipped or failed")
		h.failed(label)
	}
}
