package post

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/utils/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// injectRuntimeAnnotations writes live metrics and health data as annotations
// on the CR so the gateway can use them for metrics-level and health-level gating
// in validation and mutation rules as well as preReconcile gates, then patches
// them to the cluster. Patch failures are logged and do not fail the post phase.
//
// Metrics injection respects crossAccess; health injection respects the health endpoint flag.
func injectRuntimeAnnotations(ctx context.Context, obj domain.Object, in Input) {
	u := toUnstructured(obj)
	if u == nil {
		return
	}
	existing := u.GetAnnotations()
	before := [2]string{
		existing[labels.AnnotationCrossMetrics],
		existing[labels.AnnotationHealth],
	}

	if in.CRD.CrossAccessEnabled() && len(in.MetricsMap) > 0 {
		common.InjectMetricsAnnotation(u, in.MetricsMap)
	}
	if in.CRD.Endpoints.IsHealthEnabled() && len(in.HealthMap) > 0 {
		common.InjectHealthAnnotation(u, in.HealthMap)
	}

	after := u.GetAnnotations()
	if after[labels.AnnotationCrossMetrics] == before[0] &&
		sameHealth(after[labels.AnnotationHealth], before[1]) {
		return
	}
	if err := in.Kube.PatchAnnotations(ctx, obj, after, metav1.PatchOptions{}); err != nil {
		logger.FromContext(ctx).Warn().Err(err).
			Str("name", obj.GetName()).
			Msg("post: runtime annotation patch failed")
	}
}

// perPassHealth are the health fields that move on every reconcile.
var perPassHealth = []string{"lastReconcile", "uptime", "totalReconciles"}

// sameHealth reports whether two health annotations differ only in fields
// that move on every pass: rewriting the CR for those alone would be an API
// write and a watch event per reconcile. Their live values are on the vitals
// endpoints; the annotation carries them as of the last change in health.
func sameHealth(a, b string) bool {
	if a == b {
		return true
	}
	var ma, mb map[string]interface{}
	if json.Unmarshal([]byte(a), &ma) != nil || json.Unmarshal([]byte(b), &mb) != nil {
		return false
	}
	for _, k := range perPassHealth {
		delete(ma, k)
		delete(mb, k)
	}
	return reflect.DeepEqual(ma, mb)
}

func toUnstructured(obj domain.Object) *unstructured.Unstructured {
	res, ok := domain.ToUnstructured(obj)
	if !ok {
		return nil
	}
	return res
}
