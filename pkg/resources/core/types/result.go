package types

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Result says what a write did. Every verb is idempotent: each result below
// comes with a nil error, so a caller that only cares about failures can
// ignore it.
type Result string

const (
	Created   Result = "created"   // Create: it did not exist.
	Unchanged Result = "unchanged" // It already existed, or already matched; left alone.
	Applied   Result = "applied"   // Apply: written with server-side apply.
	Replaced  Result = "replaced"  // Apply: an immutable field changed, so it was deleted and created again.
	Deleted   Result = "deleted"   // Delete, DeleteIfOwned.
	NotFound  Result = "not found" // Delete, DeleteIfOwned: it was already gone.
	NotOwned  Result = "not owned" // DeleteIfOwned: someone else owns it; left alone.
)

// OwnedByReference reports whether obj has an owner reference to owner, by
// UID. Pass it to DeleteIfOwned to delete only what owner owns.
func OwnedByReference(owner Owner) func(metav1.Object) bool {
	return func(obj metav1.Object) bool {
		for _, ref := range obj.GetOwnerReferences() {
			if ref.UID == owner.GetUID() {
				return true
			}
		}
		return false
	}
}
