package generic

import (
	"context"
	"fmt"
	"github.com/intentd/inrun/pkg/external"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply/templates"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// runTemplates runs the CR's onCreate and onReconcile templates: their
// external calls first (.external.<name> for everything after), then their
// resource groups, written by the coordinator's templates package. It returns
// the resolver with the calls' results.
func (r *Reconciler[PTR]) runTemplates(ctx context.Context, resolver *template.Resolver, obj domain.Object, box types.OperatorBoxConfig) (*template.Resolver, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return resolver, fmt.Errorf("kubeclient not found in context")
	}

	// The resolver from the PreparedRequest already carries enriched data.
	var err error

	if t := box.EffectiveOnReconcile(); t != nil && len(t.External) > 0 {
		resolver, err = external.Run(ctx, r.crd.GVKString(), resolver, t.External, r.kube.Clientset())
		if err != nil {
			return resolver, fmt.Errorf("external calls: %w", err)
		}
	}
	if t := box.EffectiveOnCreate(); t != nil && len(t.External) > 0 {
		resolver, err = external.Run(ctx, r.crd.GVKString(), resolver, t.External, r.kube.Clientset())
		if err != nil {
			return resolver, fmt.Errorf("external calls: %w", err)
		}
	}

	if err := templates.New(r.crd, r.bp).Run(ctx, kube, resolver, obj, box); err != nil {
		return resolver, err
	}

	return resolver, nil
}
