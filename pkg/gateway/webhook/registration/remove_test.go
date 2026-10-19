package registration

import (
	"context"
	"testing"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRemoveValidatingWebhook_RemovesExisting(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset(
		&admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inrun-admission-validation",
			},
		},
	)

	err := RemoveValidatingWebhook(ctx, client, "inrun-admission-validation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, "inrun-admission-validation", metav1.GetOptions{})

	if err == nil {
		t.Fatalf("expected webhook to be deleted")
	}
}

func TestRemoveValidatingWebhook_NoErrorWhenNotFound(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset()

	err := RemoveValidatingWebhook(ctx, client, "inrun-admission-validation")
	if err != nil {
		t.Fatalf("expected no error when webhook does not exist, got: %v", err)
	}
}

func TestRemoveMutatingWebhook_RemovesExisting(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset(
		&admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inrun-admission-mutation",
			},
		},
	)

	err := removeMutatingWebhook(ctx, client, "inrun-admission-mutation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(ctx, "inrun-admission-mutation", metav1.GetOptions{})

	if err == nil {
		t.Fatalf("expected webhook to be deleted")
	}
}

func TestRemoveMutatingWebhook_NoErrorWhenNotFound(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset()

	err := removeMutatingWebhook(ctx, client, "inrun-admission-mutation")
	if err != nil {
		t.Fatalf("expected no error when webhook does not exist, got: %v", err)
	}
}
