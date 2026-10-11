package maintain

import (
	"context"
	"fmt"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/event"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
)

// Input holds everything Apply needs from the Controller.
type Input struct {
	CRD      types.CRDEntry
	Cat      *catalog.Catalog
	Kube     kubeclient.Interface
	Recorder event.Recorder

	// ForCleanup signals that the CR is about to be deleted by the cleanup gate.
	// When true, maintain sets shouldHaveProtection=false so the two-phase
	// deletion-protection label removal runs in this cycle — before the Delete call.
	ForCleanup bool
}

// Apply ensures finalizers, labels, and annotations are up to date.
// It issues Kubernetes patch calls for each category only when a diff is found.
func Apply(
	ctx context.Context,
	in Input,
	obj domain.Object,
	box types.OperatorBoxConfig,
	resolver *template.Resolver,
) error {
	if err := applyFinalizers(ctx, in, obj, box); err != nil {
		return fmt.Errorf("maintain: finalizers: %w", err)
	}
	if err := applyLabels(ctx, in, obj, resolver); err != nil {
		return fmt.Errorf("maintain: labels: %w", err)
	}
	return nil
}
