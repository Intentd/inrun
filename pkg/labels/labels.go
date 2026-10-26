package labels

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
)

//
// ────────────────────────────────────────────────────────────────────────────────
//   Deletion Protection
// ────────────────────────────────────────────────────────────────────────────────
//

// DeletionProtectionLabel marks resources that must not be deleted. Any object
// carrying:
//
//	inrun.dev/deletion-protection=true
//
// will be matched by the deletion‑protection admission webhook. This protects
// both Inrun control‑plane resources and developer‑opt‑in resources.
const (
	DeletionProtectionLabel = "inrun.dev/deletion-protection"
	// DeletionProtectionValue is the label value that enables protection.
	DeletionProtectionValue = "true"

	// StrictModeExemptKey is the label that exempts a resource from strict‑mode
	// enforcement. When present with value "true", the resource's deletion‑protection
	// label can be removed even if strictMode is enabled.
	StrictModeExemptKey   = "inrun.dev/strict-mode-exempt"
	StrictModeExemptValue = "true"

	// ReservedDomain is the label and annotation domain kept for the runtime.
	ReservedDomain = "inrun.dev"
)

// WithDeletionProtection returns a copy of m with the deletion‑protection label
// set to "true". The input map is never modified.
func WithDeletionProtection(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	out[DeletionProtectionLabel] = DeletionProtectionValue
	return out
}

// DeletionProtectionSelector returns a LabelSelector matching only the
// deletion‑protection label. Used when constructing webhook configurations.
func DeletionProtectionSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			DeletionProtectionLabel: DeletionProtectionValue,
		},
	}
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   Inrun Control‑Plane Labels
// ────────────────────────────────────────────────────────────────────────────────
//
// These labels are applied to every Inrun control‑plane resource (Deployment,
// Service, ServiceAccount, ClusterRole, ClusterRoleBinding, webhook configs,
// TLS Secret). They allow the runtime and admission webhooks to identify the
// operator’s own components.
//

var inrunResourceLabels = map[string]string{
	"app.kubernetes.io/name": "inrun",
	"app.kubernetes.io/tag":  "inrun-internal",
	DeletionProtectionLabel:  DeletionProtectionValue,
}

// InrunBaseLabels returns a copy of the standard Inrun control‑plane
// labels. Useful for generators and CLI commands that do not load Config.
func InrunBaseLabels() map[string]string {
	out := make(map[string]string, len(inrunResourceLabels))
	for k, v := range inrunResourceLabels {
		out[k] = v
	}
	return out
}

// InrunResourceLabels returns the internal label map used for control‑plane
// resources. Callers must treat the returned map as read‑only.
func InrunResourceLabels() map[string]string {
	return inrunResourceLabels
}

// InrunResourceSelector matches exactly the Inrun control‑plane resources.
// This is used by the admission webhook for mutation/validation of operator
// components (not for deletion protection — that uses DeletionProtectionSelector).
var inrunResourceSelector = &metav1.LabelSelector{
	MatchLabels: inrunResourceLabels,
}

// InrunResourceSelector returns the selector for Inrun control‑plane
// resources.
func InrunResourceSelector() *metav1.LabelSelector {
	return inrunResourceSelector
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   Ownership & Management Labels
// ────────────────────────────────────────────────────────────────────────────────
//

// Managed marks resources that Inrun actively manages.
const (
	ManagedKey = "inrun.dev/managed"

	// ManagedValue is always "true".
	ManagedValue = "true"

	// InrunOwner identifies which CR owns a generated resource. Used by
	// reconcile loops to determine whether a resource should be updated or deleted.
	InrunOwner = "inrun-owner"

	// OwnerUID holds the UID of the CR a child belongs to, whoever wrote it.
	// Unlike InrunOwner it is unique across kinds and namespaces: cleanup,
	// prune and the final sweep find a CR's children by it.
	OwnerUID = "inrun.dev/owner-uid"

	// InrunServeTarget records the effective serve surface (alias → target)
	// that was active when a child resource was created. Used to detect orphaned
	// resources after a surface switch and clean them up on the next reconcile.
	InrunServeTarget = "inrun-serve-target"

	//
	// ────────────────────────────────────────────────────────────────────────────────
	//   Annotations
	// ────────────────────────────────────────────────────────────────────────────────
	//

	// AnnotationManagedBy identifies which Inrun operator instance manages a CR.
	AnnotationManagedBy = "inrun.dev/managed-by"
	// AnnotationManagedSince records when Inrun first took ownership of a CR.
	AnnotationManagedSince = "inrun.dev/managed-since"

	//
	// ────────────────────────────────────────────────────────────────────────────────
	//   Finalizers
	// ────────────────────────────────────────────────────────────────────────────────
	//

	// CleanupFinalizer is on every CR Inrun manages. It holds a deleted CR
	// until a finalizing reconcile returns without a requeue, so reconcilers
	// can clean up first (including cluster-scoped children).
	CleanupFinalizer = "inrun.dev/cleanup"

	//
	// AnnotationCrossMetrics stores the raw metrics payload submitted as a JSON-encoded string.
	// Injected by the Runtime and used by the Gateway so the admission webhook can make it
	// available as .metrics in validation.rules — enabling metrics-level gating.
	AnnotationCrossMetrics = "inrun.dev/cross-metrics"

	// AnnotationHealth stores the raw health payload submitted as a JSON-encoded string.
	// Injected by the Runtime and used by the Gateway so the admission webhook can make it
	//  available as .health in validation.rules —
	// enabling health-level gating.
	AnnotationHealth = "inrun.dev/health"

	// ────────────────────────────────────────────────────────────────────────────────
	//   Serve Provenance Annotations
	// ────────────────────────────────────────────────────────────────────────────────
	//
	// Written by the gateway on every CR it produces. Callers never set these —
	// the gateway owns them. All three are read by the provenance notes
	// (getServeTarget, getServeAlias) exposed via the FuncMap.

	// AnnotationServeTarget records the serve target used by the caller.
	// Example: "smartapp"
	AnnotationServeTarget = "inrun.dev/serve-target"

	// AnnotationServeAlias records the alias used by the caller, when the
	// request arrived through a named alias rather than the primary target.
	// Empty when the primary target was used directly.
	// Example: "public", "internal", "v2"
	AnnotationServeAlias = "inrun.dev/serve-alias"

	// AnnotationServeIntent stores the raw intent payload submitted by the caller
	// as a JSON-encoded string. Injected by the Gateway API in target mode so the
	// admission webhook can make it available as .request in validation.rules —
	// enabling intent-level gates that fire on the caller's vocabulary before any
	// field translation has occurred.
	AnnotationServeIntent = "inrun.dev/serve-intent"

	// AnnotationServeSelectorTarget records the target that was matched by field selector.
	// Set by the gateway when a full CR is routed via fieldSelector.
	// Example: "kitchen"
	AnnotationServeSelectorTarget = "inrun.dev/serve-selector-target"

	// AnnotationServeSelector records the field selector that caused routing.
	// Set by the gateway when a full CR is routed via fieldSelector.
	// Value is a JSON-encoded map of field paths to values.
	// Example: '{"spec.mealPlan":"dinner","spec.kitchenConfig":"standard"}'
	AnnotationServeSelector = "inrun.dev/serve-selector"

	// AnnotationMigrateFieldSnapshot stores a JSON map of {migrateFromPath: oldValue}
	// captured immediately before a field-level migration patch. Used by
	// `inrun migrate rollback` to restore the previous field layout.
	AnnotationMigrateFieldSnapshot = "inrun.dev/migrate-field-snapshot"

	// AnnotationMigrateKindSnapshot stores a JSON record of a CR-level migration:
	// the source CR's identity and spec, the destination CR's identity, and whether
	// the source was pruned after the destination was applied. Used by
	// `inrun migrate rollback` to delete the destination and, if the source was
	// pruned, recreate it. Distinct from AnnotationMigrateFieldSnapshot, which records
	// a field-level migration within a single CRD.
	AnnotationMigrateKindSnapshot = "inrun.dev/migrate-kind-snapshot"

	// AnnotationMigrateTimestamp records when the most recent schema migration
	// patch was applied to this CR, as an RFC3339 timestamp.
	AnnotationMigrateTimestamp = "inrun.dev/migrate-timestamp"

	// AnnotationLastSurface records the serve surface (target name) that was active
	// on the last successful reconcile of a CR. Written by the reconciler after
	// surface orphan cleanup so that the next reconcile can detect a surface switch
	// and clean up resources from the previous surface.
	AnnotationLastSurface = "inrun.dev/last-surface"
)

// EffectiveOwnerKey returns the ownership identity to stamp on child resources
// and to use in DeleteIfOwned checks. When the owner has an active serve target
// the identity encodes both CR name and target so resources from different
// surfaces have distinct labels and orphan detection is precise.
//
// Format:
//   - target mode:  "<crName>.<target>"  e.g. "hello-website.web"
//   - direct apply: "<crName>"           e.g. "hello-website"
func EffectiveOwnerKey(ownerName string, ownerAnnotations map[string]string) string {
	if ownerAnnotations != nil {
		if t := ownerAnnotations[AnnotationServeAlias]; t != "" {
			return ownerName + "." + t
		}
		if t := ownerAnnotations[AnnotationServeTarget]; t != "" {
			return ownerName + "." + t
		}
	}
	return ownerName
}

// StampInrunLabels stamps Inrun's ownership labels for a child of owner onto
// lbls: managed, the owner's UID (OwnerUID: how Inrun finds and deletes the
// child, unique across namespaces and kinds), the readable owner key
// (InrunOwner: name, or name.target from a serve surface) and the serve
// target. Every child, from templates, hooks or a Result, is stamped here.
func StampInrunLabels(lbls map[string]string, owner metav1.Object) map[string]string {
	if lbls == nil {
		lbls = make(map[string]string)
	}
	ownerAnnotations := owner.GetAnnotations()
	lbls[ManagedKey] = ManagedValue
	lbls[InrunOwner] = EffectiveOwnerKey(owner.GetName(), ownerAnnotations)
	if uid := owner.GetUID(); uid != "" {
		lbls[OwnerUID] = string(uid)
	}
	if ownerAnnotations != nil {
		target := ownerAnnotations[AnnotationServeAlias]
		if target == "" {
			target = ownerAnnotations[AnnotationServeTarget]
		}
		if target != "" {
			lbls[InrunServeTarget] = target
		}
	}
	return lbls
}

// OwnedBy reports whether obj carries owner's UID label: whether Inrun may
// delete it as owner's. The UID, not the name, so a CR never matches another
// CR's children of the same name in another namespace. Pass it to a resource's
// DeleteIfOwned.
func OwnedBy(owner domain.Object) func(metav1.Object) bool {
	uid := string(owner.GetUID())
	return func(obj metav1.Object) bool {
		return uid != "" && obj.GetLabels()[OwnerUID] == uid
	}
}
