package validate

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/admission"
	"github.com/intentd/inrun/pkg/catalog/validate/crds"
	"github.com/intentd/inrun/pkg/catalog/validate/emit"
	"github.com/intentd/inrun/pkg/catalog/validate/gateway"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/catalog/validate/metadata"
	"github.com/intentd/inrun/pkg/catalog/validate/notes"
	"github.com/intentd/inrun/pkg/catalog/validate/observe"
	"github.com/intentd/inrun/pkg/catalog/validate/profiles"
	"github.com/intentd/inrun/pkg/catalog/validate/publish"
	"github.com/intentd/inrun/pkg/catalog/validate/reconcile"
	"github.com/intentd/inrun/pkg/catalog/validate/runtime"
	"github.com/intentd/inrun/pkg/catalog/validate/security"
	"github.com/intentd/inrun/pkg/catalog/validate/serve"
	"github.com/intentd/inrun/pkg/config"
)

// blocks are the Catalog's blocks in the order its schema reference documents
// them. Checks read only what the user declared and write nothing but
// warnings, so the order between blocks is free; the first error stops the run.
var blocks = []func(*catalog.Catalog, *config.Config) error{
	metadata.Check,
	crds.Check,
	reconcile.Check,
	runtime.Check,
	observe.Check,
	emit.Check,
	admission.Check,
	security.Check,
	notes.Check,
	profiles.Check,
	serve.Check,
	gateway.Check,
	publish.Check,
}

// Execute validates cat block by block, then fills in defaults. cat must have
// been populated by BuildRuntimeCatalog.
func Execute(cat *catalog.Catalog, cfg *config.Config) error {
	if err := config.Validate().Struct(cat); err != nil {
		check.HandleValidationErrors(err)
		return err
	}
	for _, check := range blocks {
		if err := check(cat, cfg); err != nil {
			return err
		}
	}
	return applyDefaults(cat)
}

// DetectCycles reports a dependsOn cycle between CRDs.
func DetectCycles(cat *catalog.Catalog) error { return crds.DetectCycles(cat) }

// ValidateGatewayClusters checks gateway.clusters and the serve targets that
// reference them.
func ValidateGatewayClusters(cat *catalog.Catalog) error { return gateway.Clusters(cat) }

// ValidateServe checks every CRD's serve configuration.
func ValidateServe(cat *catalog.Catalog) error { return serve.Validate(cat) }
