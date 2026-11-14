// pkg/children/builtins.go
//
// Single authoritative registry for every Kubernetes built-in resource kind
// Inrun knows about. Adding one entry here is the only change required to:
//
//   - Resolve GVR for children and GVR lookups
//   - Generate RBAC rules (ClusterRole) for any blueprint that uses the resource
//   - Detect usage in onReconcile / onCreate / onDelete template blocks
//   - Expand kind shorthands (e.g. "hpa" → "horizontalpodautoscaler")
//   - Derive the canonical PascalCase Kind name
//   - Drive readiness and deletion-protection logic
//
// Keys are lowercase singular Kind names (e.g. "deployment", "namespace").
// Accessor functions live in builtins_accessors.go.
//
// NOTE: If a new kind supports context enrichment, also add an entry to
// enrichmentMeta (in this same file) — it is intentionally a parallel map so
// enrichment concerns stay separate from Kubernetes API identity.
package children

import (
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/builtins"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BuiltInKind is a Kubernetes built-in kind as Inrun uses it: its API
// identity, shorthands and status traits (builtins.Def), and Inrun's own
// columns.
type BuiltInKind struct {
	builtins.Def

	// Detect reports whether a CRD's operatorBox uses this resource in any
	// hook template block (onCreate / onReconcile / onDelete).
	// nil for resources that cannot appear in hook templates (e.g. Node, Event).
	Detect func(crd types.CRDEntry) bool

	IsChild       bool // Inrun may create this as a child resource
	InrunInternal bool // Part of Inrun's own control-plane installation

	// HookKey is the YAML key used in HookTemplates for this resource type
	// (e.g. "networkPolicies", "deployments", "hpa"). Non-empty only for
	// IsChild entries that have a native HookTemplates field. Used by validation
	// to redirect custom: declarations to the correct key.
	HookKey string
}

// enrichmentEntry defines how a resource kind participates in context enrichment.
// Target marks the kind as a valid enrich: target. EnrichKeys lists any synthetic
// enrichment identifiers the resource provides beyond its own name/plural/shorthands
// (e.g. "replicasets" on a Deployment).
type enrichmentEntry struct {
	Target     bool
	EnrichKeys []string
}

// enrichmentMeta maps each canonical kind name to its enrichment configuration.
// This is a sibling of builtInRegistry — update both when adding a resource kind
// that should be reachable via enrich: in a blueprint spec.
var enrichmentMeta = map[string]enrichmentEntry{
	"pod":                     {Target: true},
	"service":                 {Target: true, EnrichKeys: []string{"backingpods"}},
	"persistentvolumeclaim":   {Target: true},
	"persistentvolume":        {Target: true},
	"event":                   {Target: true, EnrichKeys: []string{"warnings"}},
	"node":                    {Target: true},
	"deployment":              {Target: true, EnrichKeys: []string{"replicasets"}},
	"statefulset":             {Target: true, EnrichKeys: []string{"pvcs"}},
	"replicaset":              {Target: true},
	"job":                     {Target: true},
	"cronjob":                 {Target: true},
	"ingress":                 {Target: true},
	"networkpolicy":           {Target: true},
	"horizontalpodautoscaler": {Target: true},
	"storageclass":            {Target: true},
	"endpointslice":           {Target: true, EnrichKeys: []string{"endpoints"}},
}

// detectAny returns true if any hook template block in the CRD uses the
// resource selected by sel.
func detectAny[T any](crd types.CRDEntry, sel func(*types.HookTemplates) []T) bool {
	rc := crd.OperatorBox
	return types.UsesTemplates(rc.EffectiveOnCreate(), sel) ||
		types.UsesTemplates(rc.EffectiveOnReconcile(), sel) ||
		types.UsesTemplates(rc.EffectiveOnDelete(), sel)
}

// inrunColumns are Inrun's columns of the built-in kinds that have any,
// keyed like the builtins table.
var inrunColumns = map[string]BuiltInKind{
	"pod": {
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PodTemplateSource { return t.Pods })
		},
	},

	"service": {
		IsChild:       true,
		InrunInternal: true,
		HookKey:       "services",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ServiceTemplateSource { return t.Services })
		},
	},

	"configmap": {
		IsChild:       true,
		InrunInternal: true,
		HookKey:       "configMaps",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ConfigMapTemplateSource { return t.ConfigMaps })
		},
	},

	"secret": {
		IsChild: true,
		HookKey: "secrets",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.SecretTemplateSource { return t.Secrets })
		},
	},

	"namespace": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "namespaces",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.NamespaceTemplateSource { return t.Namespaces })
		},
	},

	"serviceaccount": {
		IsChild:       true,
		InrunInternal: true,
		HookKey:       "serviceAccounts",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ServiceAccountTemplateSource { return t.ServiceAccounts })
		},
	},

	"persistentvolumeclaim": {
		HookKey: "persistentVolumeClaims",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PVCTemplateSource { return t.PersistentVolumeClaims })
		},
	},

	"persistentvolume": {
		HookKey: "persistentVolumes",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PVTemplateSource { return t.PersistentVolumes })
		},
	},

	"resourcequota": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "resourceQuotas",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ResourceQuotaTemplateSource { return t.ResourceQuotas })
		},
	},

	"limitrange": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "limitRanges",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.LimitRangeTemplateSource { return t.LimitRanges })
		},
	},

	"deployment": {
		IsChild:       true,
		InrunInternal: true,
		HookKey:       "deployments",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.DeploymentTemplateSource { return t.Deployments })
		},
	},

	"statefulset": {
		HookKey: "statefulSets",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.StatefulSetTemplateSource { return t.StatefulSets })
		},
	},

	"replicaset": {
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ReplicaSetTemplateSource { return t.ReplicaSets })
		},
	},

	"job": {
		IsChild: true,
		HookKey: "jobs",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.JobTemplateSource { return t.Jobs })
		},
	},

	"cronjob": {
		IsChild: true,
		HookKey: "cronJobs",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.CronJobTemplateSource { return t.CronJobs })
		},
	},

	"ingress": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "ingresses",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.IngressTemplateSource { return t.Ingresses })
		},
	},

	"networkpolicy": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "networkPolicies",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.NetworkPolicyTemplateSource { return t.NetworkPolicies })
		},
	},

	"horizontalpodautoscaler": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "hpa",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.HPATemplateSource { return t.HorizontalPodAutoscalers })
		},
	},

	"role": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "roles",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.RoleTemplateSource { return t.Roles })
		},
	},

	"rolebinding": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "roleBindings",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.RoleBindingTemplateSource { return t.RoleBindings })
		},
	},

	"clusterrole": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "clusterRoles",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ClusterRoleTemplateSource { return t.ClusterRoles })
		},
	},

	"clusterrolebinding": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "clusterRoleBindings",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ClusterRoleBindingTemplateSource {
				return t.ClusterRoleBindings
			})
		},
	},

	"poddisruptionbudget": {
		InrunInternal: true,
		IsChild:       true,
		HookKey:       "pdb",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PDBTemplateSource { return t.PodDisruptionBudgets })
		},
	},

	"customresourcedefinition": {
		InrunInternal: true,
	},

	"mutatingwebhookconfiguration": {
		InrunInternal: true,
	},

	"validatingwebhookconfiguration": {
		InrunInternal: true,
	},

	"lease": {
		InrunInternal: true,
	},
}

// builtInRegistry is every built-in kind: the builtins table joined with
// Inrun's columns.
var builtInRegistry = func() map[string]BuiltInKind {
	out := map[string]BuiltInKind{}
	for key, def := range builtins.All() {
		b := inrunColumns[key]
		b.Def = def
		out[key] = b
	}
	return out
}()

func (b BuiltInKind) APIVersion() schema.GroupVersion {
	gvk, _ := GVKForBuiltInByKind(b.Kind)
	return gvk.GroupVersion()
}

// shorthandIndex maps each shorthand alias to its canonical registry key.
// Built once at init from the Shorthands field of each registry entry.
var shorthandIndex map[string]string

func init() {
	shorthandIndex = make(map[string]string, 32)
	for key, b := range builtInRegistry {
		for _, sh := range b.Shorthands {
			shorthandIndex[sh] = key
		}
	}
}
