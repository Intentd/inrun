// Secret rotation — time-based credential renewal.
//
// Extends the existing once: true pattern with a rotateAfter duration.
// When rotateAfter is set, the secret is recreated when its age exceeds
// the declared duration. The creation time is tracked via an annotation
// on the Secret itself.
//
// ── How it works ──────────────────────────────────────────────────────────
//
//  First reconcile (Secret does not exist):
//    → generate credentials, create Secret
//    → annotate with inrun.dev/generated-at: <RFC3339 timestamp>
//
//  Subsequent reconciles (Secret exists):
//    → read inrun.dev/generated-at annotation
//    → if age < rotateAfter: no-op (preserve credentials)
//    → if age >= rotateAfter: delete Secret, recreate with new credentials
//       then re-annotate with new timestamp
//
//  This is idempotent: the check-then-act pattern means no rotation happens
//  unless the threshold is crossed. The annotation is the source of truth.
//
// ── YAML ──────────────────────────────────────────────────────────────────
//
//  secrets:
//    - name: "{{ .metadata.name }}-credentials"
//      once: true
//      rotateAfter: 90d      # rotate every 90 days
//      data:
//        password: "{{ randomAlphanumeric 32 }}"
//        apiKey:   "{{ randomHex 16 }}"
//
//  # TLS certificates — auto-generated, stored as standard tls Secret type
//  secrets:
//    - name: "{{ .metadata.name }}-tls"
//      once: true
//      rotateAfter: 1y       # rotate annually
//      tls:
//        commonName: "{{ .metadata.name }}.{{ .metadata.namespace }}.svc"
//        dnsNames:
//          - "{{ .metadata.name }}"
//          - "{{ .metadata.name }}.{{ .metadata.namespace }}"
//          - "{{ .metadata.name }}.{{ .metadata.namespace }}.svc"
//          - "{{ .metadata.name }}.{{ .metadata.namespace }}.svc.cluster.local"
//        validFor: 1y
//      # Produces a Secret with:
//      #   tls.crt — PEM certificate
//      #   tls.key — PEM private key
//      #   ca.crt  — PEM CA certificate (self-signed CA)
//
// ── Duration format ───────────────────────────────────────────────────────
//
//  Supported: s (seconds), m (minutes), h (hours), d (days), y (years)
//  Examples: 30s, 5m, 12h, 90d, 1y, 365d
//  Note: d and y are extensions beyond Go's time.ParseDuration (which stops at h).
//  ParseTimeDuration handles d and y by converting to hours.
//
// ── Webhook certificate support ───────────────────────────────────────────
//
//  In the Blueprint-level webhooks block:
//
//  webhooks:
//    createCerts: true         # false by default — Inrun generates certs
//    certSecret: inrun-tls  # default name
//    rotateAfter: 1y           # default: 1 year
//
//  When createCerts: true, Inrun generates a self-signed CA, signs a
//  certificate for the webhook service, and stores both in the named Secret.
//  The webhook configuration's caBundle is patched automatically.
//  On each reconcile, Inrun checks the rotation threshold and renews
//  if needed — the caBundle patch happens alongside renewal.
//
// ── Annotation ────────────────────────────────────────────────────────────
//
//  All Inrun-generated secrets receive:
//    inrun.dev/generated-at: "2026-04-06T08:00:00Z"
//    inrun.dev/rotate-after: "90d"
//
//  These annotations are the source of truth for rotation decisions.
//  Do not remove them manually — doing so will cause regeneration on next reconcile.

package labels

import (
	"time"

	"github.com/intentd/kel/core"
)

const (
	// AnnotationGeneratedAt is the RFC3339 time a Secret was last generated.
	AnnotationGeneratedAt = "inrun.dev/generated-at"

	// AnnotationRotateAfter is the rotation duration a Secret was generated with.
	AnnotationRotateAfter = "inrun.dev/rotate-after"
)

// GenerationAnnotations returns the annotations a freshly generated Secret
// carries: when it was generated and, when it rotates, after how long.
func GenerationAnnotations(rotateAfter string) map[string]string {
	annotations := map[string]string{
		AnnotationGeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if rotateAfter != "" {
		annotations[AnnotationRotateAfter] = rotateAfter
	}
	return annotations
}

// NeedsRotation returns true when the secret has exceeded its rotation threshold.
// generatedAt is the value of the AnnotationGeneratedAt annotation.
// rotateAfter is the declared rotation duration string.
func NeedsRotation(generatedAt, rotateAfter string) bool {
	if rotateAfter == "" {
		return false // no rotation declared
	}

	t, err := time.Parse(time.RFC3339, generatedAt)
	if err != nil {
		// Cannot parse annotation — regenerate to be safe
		return true
	}

	threshold, err := core.ParseTimeDuration(rotateAfter)
	if err != nil {
		return false // invalid duration — do not rotate unexpectedly
	}

	return time.Since(t) >= threshold
}
