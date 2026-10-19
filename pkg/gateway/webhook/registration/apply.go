package registration

import (
	"context"
	"reflect"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// applyValidating creates or updates a ValidatingWebhookConfiguration. It
// skips the update when nothing Inrun controls has changed: every update fires
// a MODIFIED event, and the housekeeper would otherwise sync in a loop.
func applyValidating(ctx context.Context, client kubernetes.Interface, cfg *admissionv1.ValidatingWebhookConfiguration) error {
	api := client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	existing, err := api.Get(ctx, cfg.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = api.Create(ctx, cfg, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			// Another replica created it first.
			existing, getErr := api.Get(ctx, cfg.Name, metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}
			cfg.ResourceVersion = existing.ResourceVersion
			_, err = api.Update(ctx, cfg, metav1.UpdateOptions{})
		}
		return err
	}
	if err != nil {
		return err
	}
	if reflect.DeepEqual(existing.Labels, cfg.Labels) && reflect.DeepEqual(existing.Webhooks, cfg.Webhooks) {
		return nil
	}
	cfg.ResourceVersion = existing.ResourceVersion
	_, err = api.Update(ctx, cfg, metav1.UpdateOptions{})
	return err
}

// applyMutating is applyValidating for a MutatingWebhookConfiguration.
func applyMutating(ctx context.Context, client kubernetes.Interface, cfg *admissionv1.MutatingWebhookConfiguration) error {
	api := client.AdmissionregistrationV1().MutatingWebhookConfigurations()
	existing, err := api.Get(ctx, cfg.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = api.Create(ctx, cfg, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if reflect.DeepEqual(existing.Labels, cfg.Labels) && reflect.DeepEqual(existing.Webhooks, cfg.Webhooks) {
		return nil
	}
	cfg.ResourceVersion = existing.ResourceVersion
	_, err = api.Update(ctx, cfg, metav1.UpdateOptions{})
	return err
}

// RemoveValidatingWebhook deletes the named ValidatingWebhookConfiguration.
// A configuration that is already gone is not an error.
func RemoveValidatingWebhook(ctx context.Context, client kubernetes.Interface, name string) error {
	api := client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	if _, err := api.Get(ctx, name, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return api.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: ptr.To(gracePeriodSeconds)})
}

// removeMutatingWebhook is RemoveValidatingWebhook for a MutatingWebhookConfiguration.
func removeMutatingWebhook(ctx context.Context, client kubernetes.Interface, name string) error {
	api := client.AdmissionregistrationV1().MutatingWebhookConfigurations()
	if _, err := api.Get(ctx, name, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return api.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: ptr.To(gracePeriodSeconds)})
}
