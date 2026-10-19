package housekeeper

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// syncTokenSecrets reloads the Gateway API tokens when a token Secret is
// missing; the reload creates it again. Only the ticker catches this: the
// tokens stay in memory, so requests keep working meanwhile.
func (h *Housekeeper) syncTokenSecrets() {
	if h.cfg == nil || !h.bp.HasGatewayAPISecretRefs() {
		return
	}
	ownNS := h.cfg.Cluster().Namespace()

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	missing := false
	for _, t := range h.bp.Gateway.API.Auth.Tokens {
		if t.SecretRef == nil {
			continue
		}
		ns := t.SecretRef.Namespace
		if ns == "" {
			ns = ownNS
		}
		_, err := h.client.CoreV1().Secrets(ns).Get(ctx, t.SecretRef.Name, metav1.GetOptions{})
		switch {
		case err == nil:
		case k8serrors.IsNotFound(err):
			logger.Warn().Str("secret", t.SecretRef.Name).Str("namespace", ns).Str("token", t.Name).
				Msg("housekeeper: gateway API token secret missing, reloading")
			missing = true
		default:
			logger.Error().Err(err).Str("secret", t.SecretRef.Name).Str("namespace", ns).
				Msg("housekeeper: failed to check gateway API token secret")
			metrics.RecordWebhookReconciliationFailure(labelTokenSecret)
		}
	}

	if !missing || h.reloadTokens == nil {
		return
	}
	if err := h.reloadTokens(ctx); err != nil {
		logger.Error().Err(err).Msg("housekeeper: gateway API token reload failed")
		metrics.RecordWebhookReconciliationFailure(labelTokenSecret)
		return
	}
	logger.Info().Msg("housekeeper: gateway API tokens reloaded")
	metrics.RecordWebhookReconciled(labelTokenSecret)
}
