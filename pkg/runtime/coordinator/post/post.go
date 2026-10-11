package post

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ValidationResult is the subset of prepare.ValidationResult that post needs
// for writing the ValidationFailed/ValidationWarning conditions.
type ValidationResult struct {
	Deny       bool
	Violations []ValidationViolation
	Warnings   []ValidationViolation
}

// ValidationViolation mirrors prepare.ValidationViolation for status conditions.
type ValidationViolation struct {
	Field   string
	Rule    string
	Value   string
	Message string
}

// Input holds everything Apply needs.
type Input struct {
	CRD      types.CRDEntry
	Kube     kubeclient.Interface
	Recorder event.Recorder
	// MetricsMap carries live queue/worker/autoscale metrics for resolver enrichment
	// and metrics annotation injection. Nil is safe — enrichment is skipped.
	MetricsMap map[string]interface{}
	// HealthMap carries live CRD health data for resolver enrichment and health
	// annotation injection. Nil is safe — enrichment is skipped.
	HealthMap map[string]interface{}
	// Status is the reconciler's Result.Status. Merged after emit.status
	// fields, so the reconciler's fields win on conflict; one patch is sent.
	// Nil or empty is safe — skipped.
	Status map[string]interface{}
}

// Apply runs the full post-reconcile phase.
//
// reconcileErr is nil on success; it drives the Ready condition and layer-2 field gating.
// valResult may be nil — when nil, validation conditions are written as Passed.
func Apply(
	ctx context.Context,
	in Input,
	obj domain.Object,
	resolver *template.Resolver,
	box types.OperatorBoxConfig,
	reconcileErr error,
	valResult *ValidationResult,
) {
	// Extend resolver with child state for status field templates.
	if reconcileErr == nil && (box.EffectiveOnCreate() != nil || box.EffectiveOnReconcile() != nil) {
		ch := children.ReadChildren(ctx, in.Kube, obj, resolver, in.CRD)
		resolver = resolver.WithChildren(ch)
	}

	// Enrich resolver with live runtime metrics and health so status.fields
	// templates can reference .metrics.queueDepth, .health.state, etc.
	if len(in.MetricsMap) > 0 {
		resolver = resolver.WithMetrics(in.MetricsMap)
	}
	if len(in.HealthMap) > 0 {
		resolver = resolver.WithHealth(in.HealthMap)
	}

	// Annotate the object with runtime metrics/health
	// for gateway and preReconcile gating.
	injectRuntimeAnnotations(ctx, obj, in)

	applyStatus(ctx, in, obj, resolver, reconcileErr, valResult, box)
	applyEmit(ctx, in, obj, resolver, box, reconcileErr)
}
