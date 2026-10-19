package validate

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/admission"
	"github.com/intentd/inrun/pkg/blueprint/validate/crds"
	"github.com/intentd/inrun/pkg/blueprint/validate/emit"
	"github.com/intentd/inrun/pkg/blueprint/validate/function"
	"github.com/intentd/inrun/pkg/blueprint/validate/gateway"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/blueprint/validate/metadata"
	"github.com/intentd/inrun/pkg/blueprint/validate/observe"
	"github.com/intentd/inrun/pkg/blueprint/validate/presets"
	"github.com/intentd/inrun/pkg/blueprint/validate/publish"
	"github.com/intentd/inrun/pkg/blueprint/validate/reconcile"
	"github.com/intentd/inrun/pkg/blueprint/validate/runtime"
	"github.com/intentd/inrun/pkg/blueprint/validate/security"
	"github.com/intentd/inrun/pkg/blueprint/validate/serve"
	"github.com/intentd/inrun/pkg/config"
)

// blocks are the Blueprint's blocks in the order its schema reference documents
// them. Checks read only what the user declared and write nothing but
// warnings, so the order between blocks is free; the first error stops the run.
var blocks = []func(*blueprint.Blueprint, *config.Config) error{
	metadata.Check,
	crds.Check,
	reconcile.Check,
	runtime.Check,
	observe.Check,
	emit.Check,
	admission.Check,
	security.Check,
	function.Check,
	presets.Check,
	serve.Check,
	gateway.Check,
	publish.Check,
}

// Execute validates bp block by block, then fills in defaults. bp must have
// been populated by BuildRuntimeBlueprint.
func Execute(bp *blueprint.Blueprint, cfg *config.Config) error {
	if err := config.Validate().Struct(bp); err != nil {
		check.HandleValidationErrors(err)
		return err
	}
	for _, check := range blocks {
		if err := check(bp, cfg); err != nil {
			return err
		}
	}
	return applyDefaults(bp)
}

// DetectCycles reports a dependsOn cycle between CRDs.
func DetectCycles(bp *blueprint.Blueprint) error { return crds.DetectCycles(bp) }

// ValidateGatewayClusters checks gateway.clusters and the serve targets that
// reference them.
func ValidateGatewayClusters(bp *blueprint.Blueprint) error { return gateway.Clusters(bp) }

// ValidateServe checks every CRD's serve configuration.
func ValidateServe(bp *blueprint.Blueprint) error { return serve.Validate(bp) }
