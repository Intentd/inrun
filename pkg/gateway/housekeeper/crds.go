package housekeeper

import (
	"context"
	"encoding/base64"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
)

// CRDWatcher watches one CRD by name. The gateway app implements it with the
// apiextensions client, which this package does not import.
type CRDWatcher interface {
	Watch(ctx context.Context, crdName string) (watch.Interface, error)
}

// ConversionCRDPatchFn sets the caBundle on one CRD's conversion webhook.
type ConversionCRDPatchFn func(ctx context.Context, crdName, caBundle64 string) error

// SetConversionCRDPatcher gives the housekeeper the patch that restores a
// conversion CRD's caBundle.
func (h *Housekeeper) SetConversionCRDPatcher(fn ConversionCRDPatchFn) {
	h.patchConversionCRD = fn
}

// SetCRDWatcher gives the housekeeper a way to watch conversion CRDs. Without
// one, a stripped caBundle is restored at the next tick instead of at once.
func (h *Housekeeper) SetCRDWatcher(w CRDWatcher) {
	h.crdWatcher = w
}

// manageConversion reports whether the housekeeper owns the conversion
// caBundles. A configured certificate's owner configures spec.conversion.
func (h *Housekeeper) manageConversion() bool {
	return h.bp.IsConversionEnabled() && h.cert != nil
}

// syncConversionCRDs patches the caBundle on every conversion CRD. Patching an
// already correct CRD changes nothing and fires no MODIFIED event, so the watch
// does not loop.
func (h *Housekeeper) syncConversionCRDs() {
	if !h.manageConversion() || h.patchConversionCRD == nil {
		return
	}
	caBundle64 := base64.StdEncoding.EncodeToString(h.cert.caPEM)

	ctx, cancel := context.WithTimeout(context.Background(), highTimeout)
	defer cancel()

	for _, name := range h.bp.ManagedConversionCRDs() {
		if err := h.patchConversionCRD(ctx, name, caBundle64); err != nil {
			logger.Error().Err(err).Str("crd", name).Msg("housekeeper: failed to patch CRD conversion caBundle")
			metrics.RecordWebhookReconciliationFailure(labelConversionCABundle)
			continue
		}
		logger.Debug().Str("crd", name).Msg("housekeeper: CRD conversion caBundle in sync")
	}
}

// watchConversionCRDs watches each conversion CRD, so a stripped caBundle is
// restored within one API round-trip.
func (h *Housekeeper) watchConversionCRDs(ctx context.Context, trigger chan<- struct{}) {
	if !h.manageConversion() || h.crdWatcher == nil {
		return
	}
	for _, name := range h.bp.ManagedConversionCRDs() {
		go watchLoop(ctx, trigger, "conversion-crd "+name, watch.Modified, func(ctx context.Context) (watch.Interface, error) {
			return h.crdWatcher.Watch(ctx, name)
		})
	}
}
