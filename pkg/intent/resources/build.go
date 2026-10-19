package resources

import (
	"fmt"
	"sort"
	"strings"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/intentd/inrun/domain"
)

// resourceBuilder builds the object map for one kind from its intent fields,
// owned by owner.
type resourceBuilder func(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error)

// resourceBuilders is the one list of kinds an intent can name.
var resourceBuilders = map[string]resourceBuilder{
	"configmap":      BuildConfigMap,
	"cronjob":        BuildCronJob,
	"custom":         BuildCustomResource,
	"deployment":     BuildDeployment,
	"ingress":        BuildIngress,
	"job":            BuildJob,
	"secret":         BuildSecret,
	"serviceaccount": BuildServiceAccount,
	"service":        BuildService,
	"statefulset":    BuildStatefulSet,
	"namespace":      BuildNamespace,
	"pdb":            BuildPDB,
}

// Build builds the object a resource intent of type typ describes, owned by
// owner, with the builder for that type.
func Build(typ string, fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	if owner == nil {
		return nil, fmt.Errorf("intent form requires the owner object")
	}
	build, ok := resourceBuilders[typ]
	if !ok {
		return nil, fmt.Errorf(
			"unknown resource type %q — use full apiVersion/kind form or one of: %s",
			typ, strings.Join(Types(), ", "),
		)
	}
	return build(fields, owner)
}

// Types returns the types a resource intent can name, sorted.
func Types() []string {
	types := make([]string, 0, len(resourceBuilders))
	for t := range resourceBuilders {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func resolveConcurrencyPolicy(s string) batchv1.ConcurrencyPolicy {
	switch strings.ToLower(s) {
	case "forbid":
		return batchv1.ForbidConcurrent
	case "replace":
		return batchv1.ReplaceConcurrent
	default:
		return batchv1.AllowConcurrent
	}
}
