package registration

import (
	"context"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/labels"
)

// RegisterDeletionProtectionWebhook creates or updates the deletion-protection
// configuration. It holds up to two webhooks: one for DELETE on CRDs, and one
// for DELETE on labeled Inrun resources (deployments, services, webhook
// configurations), narrowed by ObjectSelector.
func RegisterDeletionProtectionWebhook(ctx context.Context, client kubernetes.Interface, gvrs []blueprint.GVREntry, caBundle []byte, opts Options) error {
	var crdGVRs, inrunGVRs []blueprint.GVREntry
	for _, gvr := range gvrs {
		if gvr.Resource == "customresourcedefinitions" {
			crdGVRs = append(crdGVRs, gvr)
		} else {
			inrunGVRs = append(inrunGVRs, gvr)
		}
	}

	webhook := func(name string, gvrs []blueprint.GVREntry) admissionv1.ValidatingWebhook {
		return admissionv1.ValidatingWebhook{
			Name: name,
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  serviceRef(opts, "/deletion-protection"),
				CABundle: caBundle,
			},
			Rules:                   buildRules(gvrs),
			FailurePolicy:           ptr.To(admissionv1.Fail),
			MatchPolicy:             ptr.To(admissionv1.Exact),
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](5),
		}
	}

	webhooks := make([]admissionv1.ValidatingWebhook, 0, 2)
	if len(crdGVRs) > 0 {
		webhooks = append(webhooks, webhook("protect.crds.inrun.dev", crdGVRs))
	}
	if len(inrunGVRs) > 0 {
		w := webhook("protect.resources.inrun.dev", inrunGVRs)
		w.ObjectSelector = labels.DeletionProtectionSelector()
		webhooks = append(webhooks, w)
	}

	config := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   DeletionProtectionConfigName,
			Labels: opts.InrunResourceLabels,
		},
		Webhooks: webhooks,
	}

	// Replicas starting together race to create it; back off and try again.
	for i := 0; i < 5; i++ {
		err := applyValidating(ctx, client, config)
		if err == nil {
			return nil
		}
		if !errors.IsAlreadyExists(err) && !errors.IsConflict(err) {
			return err
		}
		time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
	}
	return fmt.Errorf("failed to register deletion protection webhook after retries")
}

// RegisterNamespaceProtectionWebhook creates or updates the namespace-protection
// configuration: CREATE and UPDATE on CRDs that declare allowedNamespaces or
// restrictedNamespaces, with the Blueprint's own failure policy.
func RegisterNamespaceProtectionWebhook(ctx context.Context, client kubernetes.Interface, gvrs []blueprint.GVREntry, caBundle []byte, opts Options, failurePolicy string) error {
	config := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   NamespaceProtectionConfigName,
			Labels: opts.InrunResourceLabels,
		},
		Webhooks: []admissionv1.ValidatingWebhook{{
			Name: "namespace-protect.inrun.dev",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  serviceRef(opts, "/namespace-protection"),
				CABundle: caBundle,
			},
			Rules:                   buildRules(gvrs),
			FailurePolicy:           ptr.To(FailurePolicy(failurePolicy)),
			MatchPolicy:             ptr.To(admissionv1.Exact),
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](5),
		}},
	}
	return applyValidating(ctx, client, config)
}

// RegisterStrictModeWebhook creates or updates the strict-mode configuration:
// UPDATE on any resource carrying the deletion-protection label. The selector
// matches when either the old or the new object has the label, so removing
// the label is caught too.
func RegisterStrictModeWebhook(ctx context.Context, client kubernetes.Interface, caBundle []byte, opts Options) error {
	updateAny := admissionv1.RuleWithOperations{
		Operations: []admissionv1.OperationType{admissionv1.Update},
		Rule: admissionv1.Rule{
			APIGroups:   []string{"*"},
			APIVersions: []string{"*"},
			Resources:   []string{"*"},
		},
	}

	config := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   StrictModeConfigName,
			Labels: opts.InrunResourceLabels,
		},
		Webhooks: []admissionv1.ValidatingWebhook{{
			Name: "strict-mode.inrun.dev",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  serviceRef(opts, "/strict-mode-protection"),
				CABundle: caBundle,
			},
			ObjectSelector:          labels.DeletionProtectionSelector(),
			Rules:                   []admissionv1.RuleWithOperations{updateAny},
			FailurePolicy:           ptr.To(admissionv1.Fail),
			MatchPolicy:             ptr.To(admissionv1.Equivalent),
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
			TimeoutSeconds:          ptr.To[int32](5),
		}},
	}
	return applyValidating(ctx, client, config)
}
