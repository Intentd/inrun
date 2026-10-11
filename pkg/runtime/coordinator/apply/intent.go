package apply

import (
	"fmt"
	"sort"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/configmaps"
	"github.com/intentd/inrun/pkg/resources/cronjobs"
	"github.com/intentd/inrun/pkg/resources/customresources"
	"github.com/intentd/inrun/pkg/resources/deployments"
	"github.com/intentd/inrun/pkg/resources/ingresses"
	"github.com/intentd/inrun/pkg/resources/jobs"
	"github.com/intentd/inrun/pkg/resources/namespaces"
	"github.com/intentd/inrun/pkg/resources/pdbs"
	"github.com/intentd/inrun/pkg/resources/secrets"
	"github.com/intentd/inrun/pkg/resources/serviceaccounts"
	"github.com/intentd/inrun/pkg/resources/services"
	"github.com/intentd/inrun/pkg/resources/statefulsets"
)

// intentBuilder builds a Kubernetes object from an intent form's fields,
// owned by the given object.
type intentBuilder func(map[string]interface{}, domain.Object) (map[string]interface{}, error)

// intentBuilders is the single source of truth for which resource types can be
// built from an intent form. Add a type here and it's both dispatchable and
// advertised in the "unknown resource type" error — no second list to keep in sync.
var intentBuilders = map[string]intentBuilder{
	"configmap":      configmaps.BuildFromIntent,
	"cronjob":        cronjobs.BuildFromIntent,
	"custom":         customresources.BuildFromIntent,
	"deployment":     deployments.BuildFromIntent,
	"ingress":        ingresses.BuildFromIntent,
	"job":            jobs.BuildFromIntent,
	"secret":         secrets.BuildFromIntent,
	"serviceaccount": serviceaccounts.BuildFromIntent,
	"service":        services.BuildFromIntent,
	"statefulset":    statefulsets.BuildFromIntent,
	"namespace":      namespaces.BuildFromIntent,
	"pdb":            pdbs.BuildFromIntent,
}

// buildFromIntent dispatches a resource intent to the matching builder.
// A nil owner is rejected here so no builder has to check for it.
func buildFromIntent(typ string, fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	if owner == nil {
		return nil, fmt.Errorf("intent form requires the owner object")
	}
	build, ok := intentBuilders[typ]
	if !ok {
		return nil, fmt.Errorf(
			"unknown resource type %q — use full apiVersion/kind form or one of: %s",
			typ, strings.Join(supportedIntentTypes(), ", "),
		)
	}
	return build(fields, owner)
}

// supportedIntentTypes returns the registered type names for use in error
// messages. Names are sorted for easy read.
func supportedIntentTypes() []string {
	types := make([]string, 0, len(intentBuilders))
	for t := range intentBuilders {
		types = append(types, t)
	}
	sortStrings(types)
	return types
}

func sortStrings(values []string) {
	sort.Strings(values)
}
