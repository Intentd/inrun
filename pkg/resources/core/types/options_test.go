package types

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestOptions_PatchOptions(t *testing.T) {
	got := Options{}.PatchOptions()
	if got.FieldManager != DefaultFieldManager || got.Force == nil || *got.Force {
		t.Errorf("zero Options = %+v, want the default field manager and no force", got)
	}

	got = Options{FieldManager: "inrun", Force: true}.PatchOptions()
	if got.FieldManager != "inrun" || !*got.Force {
		t.Errorf("Options = %+v, want inrun and force", got)
	}
}

func TestOwnedByReference(t *testing.T) {
	owner := &unstructured.Unstructured{}
	owner.SetUID("owner-uid")
	owned := OwnedByReference(owner)

	child := &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{UID: "owner-uid"}}}
	if !owned(child) {
		t.Error("an object with an owner reference to owner should be owned")
	}
	other := &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{UID: k8stypes.UID("someone-else")}}}
	if owned(other) {
		t.Error("an object owned by someone else should not be owned")
	}
	if owned(&metav1.ObjectMeta{}) {
		t.Error("an object without owner references should not be owned")
	}
}
