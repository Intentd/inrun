package kube

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
)

// SecretExists reports whether the Secret namespace/name exists: false when
// it is not found, an error when the API call fails. It reads from the API
// server's watch cache (ResourceVersion "0"), not etcd.
func SecretExists(ctx context.Context, cs kubernetes.Interface, namespace, name string) (bool, error) {
	_, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{ResourceVersion: "0"})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking secret %s/%s: %w", namespace, name, err)
	}
	return true, nil
}

// ReadSecretKey returns the value at key in the Secret namespace/name, read
// from the API server's watch cache (ResourceVersion "0").
func ReadSecretKey(ctx context.Context, cs kubernetes.Interface, namespace, name, key string) (string, error) {
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{ResourceVersion: "0"})
	if err != nil {
		return "", fmt.Errorf("reading secret %s/%s: %w", namespace, name, err)
	}
	val, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no key %q", namespace, name, key)
	}
	return string(val), nil
}

// DeleteSecret deletes the Secret namespace/name; one already gone is not an
// error.
func DeleteSecret(ctx context.Context, cs kubernetes.Interface, namespace, name string) error {
	err := cs.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting secret %s/%s: %w", namespace, name, err)
	}
	return nil
}

// SecretNeedsRotation reports whether the Secret namespace/name is due to be
// generated again, by its labels.AnnotationGeneratedAt and rotateAfter (see
// labels.NeedsRotation). It is false when rotateAfter is empty or the Secret
// doesn't exist yet. A Secret without the annotation, made outside Inrun, is
// annotated now and its rotation clock starts from this call.
func SecretNeedsRotation(ctx context.Context, cs kubernetes.Interface, namespace, name, rotateAfter string) (bool, error) {
	if rotateAfter == "" {
		return false, nil
	}
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{ResourceVersion: "0"})
	if apierrors.IsNotFound(err) {
		return false, nil // to be created, not rotated
	}
	if err != nil {
		return false, fmt.Errorf("checking secret %s/%s for rotation: %w", namespace, name, err)
	}

	generatedAt := secret.Annotations[labels.AnnotationGeneratedAt]
	if generatedAt == "" {
		logger.FromContext(ctx).Warn().
			Str("secret", name).
			Msg("secret missing generated-at annotation — annotating now, rotation clock starts")
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		secret.Annotations[labels.AnnotationGeneratedAt] = time.Now().UTC().Format(time.RFC3339)
		secret.Annotations[labels.AnnotationRotateAfter] = rotateAfter
		if _, err := cs.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return false, fmt.Errorf("annotating secret %s/%s: %w", namespace, name, err)
		}
		return false, nil
	}
	return labels.NeedsRotation(generatedAt, rotateAfter), nil
}
