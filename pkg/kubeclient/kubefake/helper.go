package kubefake

import (
	"github.com/intentd/inrun/pkg/children"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8stesting "k8s.io/client-go/testing"
)

// ListKindsForSeeds starts from the registry's full built-in list-kind map and
// merges the list kind for every seeded object's GVR. The registry covers
// built-ins a reconciler might List without seeding (e.g. ConfigMaps); the
// seeds cover the CRD under test and any siblings.
func ListKindsForSeeds(scheme *runtime.Scheme, seeds []runtime.Object) map[schema.GroupVersionResource]string {
	listKinds := children.ListKindsForFakeDynamic()

	for _, obj := range seeds {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			if kinds, _, err := scheme.ObjectKinds(obj); err == nil && len(kinds) > 0 {
				gvk = kinds[0]
			}
		}
		if gvk.Empty() {
			continue
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		listKinds[gvr] = gvk.Kind + "List"
	}

	return listKinds
}

// verbFromAction returns "apply" for Server-Side Apply patch operations,
// "patch" for all other patch types, and the raw verb for everything else.
func verbFromAction(action k8stesting.Action) string {
	if pa, ok := action.(k8stesting.PatchAction); ok {
		if pa.GetPatchType() == k8stypes.ApplyPatchType {
			return "apply"
		}
	}
	return action.GetVerb()
}

// nameFromAction extracts the resource name from a test action. Actions that
// name their object (get, delete, patch) satisfy GetAction; actions that carry
// it (create, update) satisfy CreateAction, so one case covers each.
func nameFromAction(action k8stesting.Action) string {
	switch a := action.(type) {
	case k8stesting.GetAction:
		return a.GetName()
	case k8stesting.CreateAction:
		if acc, ok := a.GetObject().(metav1.Object); ok {
			return acc.GetName()
		}
	}
	return ""
}

// sameKind reports whether a and b are the same kind, when both say.
func sameKind(a, b runtime.Object) bool {
	ka, kb := a.GetObjectKind().GroupVersionKind().Kind, b.GetObjectKind().GroupVersionKind().Kind
	return ka == "" || kb == "" || ka == kb
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func nameFromRuntimeObject(obj runtime.Object) string {
	if acc, ok := obj.(interface{ GetName() string }); ok {
		return acc.GetName()
	}
	return ""
}
