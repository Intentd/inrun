package registration

import (
	"context"
	"fmt"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/util/retry"
)

// AdmissionRegistry is the part of blueprint.AdmissionRegistry registration reads.
type AdmissionRegistry interface {
	ValidationGVRs() []blueprint.GVREntry
	MutationGVRs() []blueprint.GVREntry
}

// RegisterAdmissionWebhooks creates or updates the validating and mutating
// configurations for the CRDs in registry that declare rules.
func RegisterAdmissionWebhooks(ctx context.Context, client kubernetes.Interface, registry AdmissionRegistry, opts Options) error {
	caBundle, err := ReadCABundle(opts.TLSCertFile)
	if err != nil {
		return fmt.Errorf("webhook registration: reading CA bundle: %w", err)
	}

	caller := registrar
	if opts.Caller != "" {
		caller = opts.Caller
	}

	if gvrs := registry.ValidationGVRs(); len(gvrs) > 0 {
		if err := applyValidating(ctx, client, validatingConfig(gvrs, caBundle, opts)); err != nil {
			return fmt.Errorf("webhook registration: validating: %w", err)
		}
		logApplied(caller, ValidatingConfigName, "ValidatingWebhookConfiguration", len(gvrs))
	}

	if gvrs := registry.MutationGVRs(); len(gvrs) > 0 {
		if err := applyMutating(ctx, client, mutatingConfig(gvrs, caBundle, opts)); err != nil {
			return fmt.Errorf("webhook registration: mutating: %w", err)
		}
		logApplied(caller, MutatingConfigName, "MutatingWebhookConfiguration", len(gvrs))
	}

	return nil
}

// UnregisterAdmissionWebhooks removes the admission configurations opts selects.
func UnregisterAdmissionWebhooks(ctx context.Context, client kubernetes.Interface, opts CleanupOptions) error {
	retryOpts := retry.Options{Attempts: maxAttempts, Delay: delayBetweenAttempts}

	if opts.Validating {
		if err := retry.RetryBackoff(func() error {
			return RemoveValidatingWebhook(ctx, client, ValidatingConfigName)
		}, retryOpts); err != nil {
			return fmt.Errorf("webhook cleanup: validating: %w", err)
		}
		logger.Info().Str("config", ValidatingConfigName).Msg("webhook: ValidatingWebhookConfiguration unregistered")
	}

	if opts.Mutating {
		if err := retry.RetryBackoff(func() error {
			return removeMutatingWebhook(ctx, client, MutatingConfigName)
		}, retryOpts); err != nil {
			return fmt.Errorf("webhook cleanup: mutating: %w", err)
		}
		logger.Info().Str("config", MutatingConfigName).Msg("webhook: MutatingWebhookConfiguration unregistered")
	}

	return nil
}

// logApplied logs a registration at startup and stays quiet on the
// housekeeper's syncs.
func logApplied(caller, config, kind string, crds int) {
	if caller == CallerHousekeeper {
		logger.Debug().Int("crds", crds).Str("config", config).Msgf("%s: %s in sync", caller, kind)
		return
	}
	logger.Info().Int("crds", crds).Str("config", config).Msgf("%s: %s registered", caller, kind)
}

func validatingConfig(gvrs []blueprint.GVREntry, caBundle []byte, opts Options) *admissionv1.ValidatingWebhookConfiguration {
	return &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ValidatingConfigName,
			Labels: opts.InrunResourceLabels,
		},
		Webhooks: []admissionv1.ValidatingWebhook{{
			Name: "validate.inrun.dev",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  serviceRef(opts, "/validate"),
				CABundle: caBundle,
			},
			Rules:                   buildRules(gvrs),
			FailurePolicy:           ptr.To(opts.FailurePolicy),
			MatchPolicy:             ptr.To(admissionv1.Exact),
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](5),
		}},
	}
}

func mutatingConfig(gvrs []blueprint.GVREntry, caBundle []byte, opts Options) *admissionv1.MutatingWebhookConfiguration {
	return &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   MutatingConfigName,
			Labels: opts.InrunResourceLabels,
		},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "mutate.inrun.dev",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  serviceRef(opts, "/mutate"),
				CABundle: caBundle,
			},
			Rules:                   buildRules(gvrs),
			FailurePolicy:           ptr.To(opts.FailurePolicy),
			MatchPolicy:             ptr.To(admissionv1.Exact),
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](5),
			ReinvocationPolicy:      ptr.To(admissionv1.IfNeededReinvocationPolicy),
		}},
	}
}
