package coordinator

import (
	"context"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/event"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/post"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/prepare"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/vitals"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
)

// PostInput holds everything runPost needs from the worker.
type PostInput struct {
	Kube      kubeclient.Interface
	Event     event.Recorder
	Health    *vitals.CRDHealth
	CRD       types.CRDEntry
	Prepared  *domain.PreparedRequest
	Result    domain.Result
	ValResult *post.ValidationResult
}

// runPost runs the full post-reconcile sequence and returns the final reconcile error.
// Status patch and declarative event emission via post.Apply.
func runPost(ctx context.Context, in PostInput, reconcileErr error) error {
	box := prepare.BoxFrom(in.Prepared)
	resolver := in.Prepared.Context.(*template.Resolver)
	post.Apply(ctx, post.Input{
		CRD:        in.CRD,
		Kube:       in.Kube,
		Recorder:   in.Event,
		MetricsMap: in.Health.GetAutoMetrics(),
		HealthMap:  in.Health.HealthAsMap(),
		Status:     in.Result.Status,
	}, in.Prepared.Object, resolver, box, reconcileErr, in.ValResult)

	return reconcileErr
}
