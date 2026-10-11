package prepare

import (
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// effectiveBoxAndTarget returns the operative OperatorBox and resolved target
// for this reconcile cycle. The target is read from the serve-target annotation
// on the CR; falls back to the CRD-level box for direct kubectl applies.
func effectiveBoxAndTarget(crd types.CRDEntry, obj *unstructured.Unstructured) (types.OperatorBoxConfig, string) {
	target := intent.Target(obj.GetAnnotations())
	box := *crd.EffectiveOperatorBox(target)
	return box, target
}
