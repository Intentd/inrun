// pkg/secrets
//
// Shared secret lifecycle utilities used by both the runtime and the gateway.
//
// These helpers live here rather than in pkg/runtime/runners/ because the Gateway
// API also needs secret existence checks and rotation logic for token secret
// management. Extracting them here avoids a gateway → runtime import.
package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretExists checks whether a Secret with the given name exists in the namespace.
// Uses ResourceVersion: "0" to read from the API server watch cache (not etcd).
// Returns true if the secret exists, false on NotFound, error on API failure.
func SecretExists(ctx context.Context, kube kubeclient.Interface, namespace, name string) (bool, error) {
	_, err := kube.Clientset().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{
		ResourceVersion: "0", // watch cache — avoids etcd round-trip
	})
	if err != nil {
		if IsNotFoundErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking secret %s/%s: %w", namespace, name, err)
	}
	return true, nil
}

// SecretNeedsRotation returns true when the Secret exists but has exceeded
// its rotation threshold. Reads the generated-at annotation and compares
// to the declared rotateAfter duration.
//
// Returns false (no rotation) when:
//   - rotateAfter is not set
//   - Secret does not exist
//   - Generated-at annotation is missing or unparseable → regenerate to be safe
func SecretNeedsRotation(ctx context.Context, kube kubeclient.Interface, namespace, name, rotateAfter string) (bool, error) {
	if rotateAfter == "" {
		return false, nil
	}

	secret, err := kube.Clientset().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{
		ResourceVersion: "0", // watch cache
	})
	if err != nil {
		if IsNotFoundErr(err) {
			return false, nil // does not exist — needs creation, not rotation
		}
		return false, fmt.Errorf("checking secret %s/%s for rotation: %w", namespace, name, err)
	}

	generatedAt := secret.Annotations[types.AnnotationGeneratedAt]
	if generatedAt == "" {
		// Secret exists but has no annotation — was it created outside Inrun?
		// Annotate it now and start the rotation clock from this reconcile.
		logger.FromContext(ctx).Warn().
			Str("secret", name).
			Msg("secret missing generated-at annotation — annotating now, rotation clock starts")
		return false, annotateSecret(ctx, kube, namespace, name, rotateAfter)
	}

	return types.NeedsRotation(generatedAt, rotateAfter), nil
}

// DeleteSecretForRotation deletes a Secret so it can be recreated with fresh values.
// Called when SecretNeedsRotation returns true. The next create call
// then generates new credentials and annotates with the current time.
func DeleteSecretForRotation(ctx context.Context, kube kubeclient.Interface, namespace, name string) error {
	err := kube.Clientset().CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !IsNotFoundErr(err) {
		return fmt.Errorf("deleting secret %s/%s for rotation: %w", namespace, name, err)
	}
	return nil
}

// GenerationAnnotations returns the annotations to add to a freshly generated Secret.
func GenerationAnnotations(rotateAfter string) map[string]string {
	annotations := map[string]string{
		types.AnnotationGeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if rotateAfter != "" {
		annotations[types.AnnotationRotateAfter] = rotateAfter
	}
	return annotations
}

// annotateSecret writes the generated-at and rotate-after annotations onto
// an existing Secret. Used when a Secret exists but was created without annotations.
func annotateSecret(ctx context.Context, kube kubeclient.Interface, namespace, name, rotateAfter string) error {
	secret, err := kube.Clientset().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{
		ResourceVersion: "0",
	})
	if err != nil {
		return err
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[types.AnnotationGeneratedAt] = time.Now().UTC().Format(time.RFC3339)
	secret.Annotations[types.AnnotationRotateAfter] = rotateAfter
	_, err = kube.Clientset().CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// ReadSecretKey fetches a Secret and returns the value at the given key.
// Uses the watch cache (ResourceVersion: "0") to avoid an etcd round-trip.
func ReadSecretKey(ctx context.Context, cs kubernetes.Interface, namespace, name, key string) (string, error) {
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{
		ResourceVersion: "0",
	})
	if err != nil {
		return "", fmt.Errorf("reading secret %s/%s: %w", namespace, name, err)
	}
	val, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no key %q", namespace, name, key)
	}
	return string(val), nil
}

// IsNotFoundErr returns true when err is a Kubernetes 404 Not Found error.
func IsNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return containsStr404(err.Error())
}

func containsStr404(s string) bool {
	return len(s) >= 9 && (containsSubstr(s, "not found") || containsSubstr(s, "NotFound") || containsSubstr(s, "404"))
}

func containsSubstr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
