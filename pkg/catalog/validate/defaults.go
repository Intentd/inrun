package validate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/catalog/validate/runtime"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/types"
)

// applyDefaults fills in what a Catalog leaves out, once every check has
// passed. Checks read only what the user declared, so nothing here can change
// what they report, and the runtime only ever sees defaulted entries:
//
//   - mode: typed when apiTypes.location is set, dynamic otherwise
//   - runtime.ignoreStatusPatch / ignoreObservedGeneration for built-in kinds
//     that have no status subresource or observedGeneration
//   - an autoscale.profile expanded into the full autoscale spec
//   - queue.behaviour.onThreshold.drop: always on at the threshold
func applyDefaults(cat *catalog.Catalog) error {
	for name, crd := range cat.EnabledCRDs() {
		if crd.Mode == "" {
			if crd.APITypes.Location != "" {
				crd.Mode = types.CRDModeTyped
			} else {
				crd.Mode = types.CRDModeDynamic
			}
		}

		// Look up by Kind directly; unknown kinds return the zero value.
		meta := children.BuiltInMeta(crd.APITypes.Kind)
		if meta.SkipStatusSubresource || meta.SkipObservedGeneration {
			if crd.Box().Runtime == nil {
				crd.Box().Runtime = &types.RuntimeConfig{}
			}
			if meta.SkipStatusSubresource {
				crd.Box().Runtime.IgnoreStatusPatch = true
			}
			if meta.SkipObservedGeneration {
				crd.Box().Runtime.IgnoreObservedGeneration = true
			}
		}

		if crd.Box().EffectiveAutoscale() != nil && crd.HasAutoscaleProfile() {
			expanded, err := runtime.ExpandAutoscaleProfile(&crd)
			if err != nil {
				return fmt.Errorf("%s CRD %q: autoscale.profile %q expansion failed: %w",
					check.FailureMark(), name, crd.AutoScaleProfile(), err)
			}
			if crd.Box().Runtime == nil {
				crd.Box().Runtime = &types.RuntimeConfig{}
			}
			crd.Box().Runtime.Autoscale = expanded
		}

		if q := crd.QueueConfig(); !q.Empty() && q.HasBehaviour() {
			if cfg := q.Behaviour(); cfg.HasOnThreshold() {
				cfg.OnThreshold.Drop = check.BoolPtr(true)
			}
		}

		cat.EnabledCRDs()[name] = crd
	}
	return nil
}
