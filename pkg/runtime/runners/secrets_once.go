// pkg/runners/secrets_once.go
//
// once: true on secrets — idempotent random secret generation.
//
// The Kubernetes reconcile loop is called on every resync. A secret with
// random data (password, API key) would change every 30 seconds without
// the once: guard. once: true makes the create step check for existence
// first — if the Secret already exists, skip template evaluation entirely.
//
// This is the check-before-generate pattern used by cert-manager and every
// other tool that generates credentials idempotently.
//
// YAML:
//
//	secrets:
//	  - name: "{{ .metadata.name }}-credentials"
//	    once: true
//	    data:
//	      password: "{{ randomAlphanumeric 32 }}"
//	      apiKey:   "{{ randomHex 16 }}"
//	      jwtSecret: "{{ randomBase64 32 }}"
//
// Behaviour:
//   - First reconcile: Secret does not exist → evaluate templates → create
//   - Every subsequent reconcile: Secret exists → skip completely (no-op)
//   - CR deleted: Secret deleted via owner reference (garbage collection)
//
// once: true is IGNORED when update=true (onReconcile with reconcile: true).
// A secret in onReconcile should use static or deterministic template values.
// Combining once: true with reconcile: true logs a warning and skips the
// once: guard — the caller opted into continuous reconciliation.
//
// once: false (default): standard create/update behavior, unchanged.
package runners

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/secrets"
)

// SecretExists delegates to pkg/secrets.
func SecretExists(ctx context.Context, kube kubeclient.Interface, namespace, name string) (bool, error) {
	return secrets.SecretExists(ctx, kube, namespace, name)
}

// IsNotFoundErr delegates to pkg/secrets.
func IsNotFoundErr(err error) bool {
	return secrets.IsNotFoundErr(err)
}

// logOnceReconcileConflict warns when once: true and reconcile: true are
// both set on the same secret. These are contradictory: once means
// "never re-evaluate", reconcile means "always re-evaluate".
func logOnceReconcileConflict(ctx context.Context, name string) {
	logger.FromContext(ctx).Warn().
		Str("secret", name).
		Msg("secret has both once: true and reconcile: true — " +
			"once: is ignored. Remove reconcile: true to enable once: semantics, " +
			"or remove once: true if you want continuous reconciliation.")
}
