package housekeeper

import (
	"context"
	"encoding/json"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// syncNamespaceLabels puts back the Inrun labels on the operator namespace.
// The deletion-protection webhook selects by them, so without them the
// namespace itself is no longer protected.
func (h *Housekeeper) syncNamespaceLabels() {
	if !h.bp.DeletionProtectionActive() {
		return
	}
	namespace := h.operatorNamespace()
	if namespace == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), lowTimeout)
	defer cancel()

	ns, err := h.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{
		ResourceVersion: "0", // served from the watch cache
	})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error().Err(err).Str("namespace", namespace).Msg("housekeeper: failed to read namespace for label check")
			metrics.RecordWebhookReconciliationFailure(labelNamespaceLabels)
		}
		return
	}

	required := labels.InrunResourceLabels()
	if labelsPresent(ns.Labels, required) {
		return
	}

	patch, err := json.Marshal(map[string]any{"metadata": map[string]any{"labels": required}})
	if err != nil {
		logger.Error().Err(err).Msg("housekeeper: failed to marshal namespace label patch")
		metrics.RecordWebhookReconciliationFailure(labelNamespaceLabels)
		return
	}

	_, err = h.client.CoreV1().Namespaces().Patch(ctx, namespace, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error().Err(err).Str("namespace", namespace).Msg("housekeeper: failed to restore namespace labels")
		metrics.RecordWebhookReconciliationFailure(labelNamespaceLabels)
		return
	}

	logger.Warn().Str("namespace", namespace).Msg("housekeeper: namespace labels restored")
	metrics.RecordWebhookReconciled(labelNamespaceLabels)
}

// labelsPresent reports whether current has every required label with the same value.
func labelsPresent(current, required map[string]string) bool {
	for k, v := range required {
		if current[k] != v {
			return false
		}
	}
	return true
}

// operatorNamespace is the namespace the gateway runs in.
func (h *Housekeeper) operatorNamespace() string {
	if h.certNamespace != "" {
		return h.certNamespace
	}
	if h.cfg != nil {
		return h.cfg.Cluster().Namespace()
	}
	return ""
}
