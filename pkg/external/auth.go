package external

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/secrets"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// ResolveAuth resolves the credential value from an ExternalAuth declaration.
// Returns the credential string and the header name to inject it into (HTTP only).
// header defaults to "Authorization"; produces "Bearer <credential>".
// cs may be nil when secretRef is not used.
// secretRef.namespace is required — validated at inrun validate time, never defaulted here.
func ResolveAuth(ctx context.Context, auth *types.ExternalAuth, cs kubernetes.Interface) (credential, header string, err error) {
	if auth == nil {
		return "", "", nil
	}

	header = auth.Header
	if header == "" {
		header = "Authorization"
	}

	switch {
	case auth.SecretRef != nil:
		credential, err = secrets.ReadSecretKey(ctx, cs, auth.SecretRef.Namespace, auth.SecretRef.Name, auth.SecretRef.Key)
		if err != nil {
			return "", "", fmt.Errorf("auth.secretRef: %w", err)
		}

	case auth.Env != "":
		credential = os.Getenv(auth.Env)
		if credential == "" {
			return "", "", fmt.Errorf("auth.env: %q is not set or empty", auth.Env)
		}

	default:
		return "", "", fmt.Errorf("auth: exactly one of secretRef or env must be set")
	}

	return credential, header, nil
}
