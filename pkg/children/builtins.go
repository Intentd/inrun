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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BuiltInKind holds the fully-qualified API metadata for a Kubernetes
// built-in resource kind, plus Inrun-specific readiness and usage metadata.
type BuiltInKind struct {
	// ── Kubernetes API identity ───────────────────────────────────────────────

	Kind    string // PascalCase Kind name (e.g. "Deployment")
	Group   string // API group; empty string for core
	Version string // API version (e.g. "v1", "v2")
	Plural  string // plural resource name (e.g. "deployments")

	Namespaced bool   // true if resource is namespaced
	APIPath    string // "/api" for core, "/apis" otherwise

	// ── Discovery ─────────────────────────────────────────────────────────────

	// Shorthands are case-insensitive aliases resolved by LookupBuiltIn.
	// e.g. "hpa" → "horizontalpodautoscaler"
	Shorthands []string

	// ── Usage detection ───────────────────────────────────────────────────────

	// Detect reports whether a CRD's operatorBox uses this resource in any
	// hook template block (onCreate / onReconcile / onDelete).
	// nil for resources that cannot appear in hook templates (e.g. Node, Event).
	Detect func(crd types.CRDEntry) bool

	// ── Inrun readiness metadata ───────────────────────────────────────────

	Statusless             bool // No meaningful status; treat as ready on existence
	SkipStatusSubresource  bool // No /status subresource; never PATCH status
	SkipObservedGeneration bool // Has status but no observedGeneration; skip generation check
	IsChild                bool // Inrun may create this as a child resource
	InrunInternal          bool // Part of Inrun's own control-plane installation

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

// builtInRegistry is the single source of truth for all Kubernetes built-in
// resource kinds. Keys are lowercase singular Kind names.
var builtInRegistry = map[string]BuiltInKind{

	// ── Core group (group: "", apiVersion: v1) ────────────────────────────────

	"pod": {
		Kind: "Pod", Group: "", Version: "v1", Plural: "pods",
		Namespaced: true, APIPath: "/api",
		SkipObservedGeneration: true,
		Shorthands:             []string{"po"},
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PodTemplateSource { return t.Pods })
		},
	},

	"service": {
		Kind: "Service", Group: "", Version: "v1", Plural: "services",
		Namespaced: true, APIPath: "/api",
		Shorthands:             []string{"svc"},
		SkipObservedGeneration: true, IsChild: true, InrunInternal: true,
		HookKey: "services",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ServiceTemplateSource { return t.Services })
		},
	},

	"configmap": {
		Kind: "ConfigMap", Group: "", Version: "v1", Plural: "configmaps",
		Namespaced: true, APIPath: "/api",
		Statusless: true, SkipStatusSubresource: true, IsChild: true, InrunInternal: true,
		Shorthands: []string{"cm"},
		HookKey:    "configMaps",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ConfigMapTemplateSource { return t.ConfigMaps })
		},
	},

	"secret": {
		Kind: "Secret", Group: "", Version: "v1", Plural: "secrets",
		Namespaced: true, APIPath: "/api",
		Statusless: true, SkipStatusSubresource: true, IsChild: true,
		HookKey: "secrets",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.SecretTemplateSource { return t.Secrets })
		},
	},

	"namespace": {
		Kind: "Namespace", Group: "", Version: "v1", Plural: "namespaces",
		Namespaced: false, APIPath: "/api",
		SkipObservedGeneration: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"ns"},
		HookKey:    "namespaces",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.NamespaceTemplateSource { return t.Namespaces })
		},
	},

	"serviceaccount": {
		Kind: "ServiceAccount", Group: "", Version: "v1", Plural: "serviceaccounts",
		Namespaced: true, APIPath: "/api",
		Statusless: true, SkipStatusSubresource: true, IsChild: true, InrunInternal: true,
		Shorthands: []string{"sa"},
		HookKey:    "serviceAccounts",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ServiceAccountTemplateSource { return t.ServiceAccounts })
		},
	},

	"persistentvolumeclaim": {
		Kind: "PersistentVolumeClaim", Group: "", Version: "v1", Plural: "persistentvolumeclaims",
		Namespaced: true, APIPath: "/api",
		SkipObservedGeneration: true,
		Shorthands:             []string{"pvc", "pvcs", "pvclaim"},
		HookKey:                "persistentVolumeClaims",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PVCTemplateSource { return t.PersistentVolumeClaims })
		},
	},

	"persistentvolume": {
		Kind: "PersistentVolume", Group: "", Version: "v1", Plural: "persistentvolumes",
		Namespaced: false, APIPath: "/api",
		SkipObservedGeneration: true,
		Shorthands:             []string{"pv", "pvs"},
		HookKey:                "persistentVolumes",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PVTemplateSource { return t.PersistentVolumes })
		},
	},

	"event": {
		Kind: "Event", Group: "", Version: "v1", Plural: "events",
		Namespaced: true, APIPath: "/api",
		Shorthands: []string{"ev"},
		Statusless: true, SkipStatusSubresource: true,
	},

	"node": {
		Kind: "Node", Group: "", Version: "v1", Plural: "nodes",
		Namespaced: false, APIPath: "/api",
		SkipObservedGeneration: true,
	},

	"resourcequota": {
		Kind: "ResourceQuota", Group: "", Version: "v1", Plural: "resourcequotas",
		Namespaced: true, APIPath: "/api",
		SkipObservedGeneration: true, InrunInternal: true, IsChild: true,
		HookKey: "resourceQuotas",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ResourceQuotaTemplateSource { return t.ResourceQuotas })
		},
	},

	"limitrange": {
		Kind: "LimitRange", Group: "", Version: "v1", Plural: "limitranges",
		Namespaced: true, APIPath: "/api",
		SkipObservedGeneration: true, InrunInternal: true, IsChild: true,
		HookKey: "limitRanges",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.LimitRangeTemplateSource { return t.LimitRanges })
		},
	},

	"componentstatus": {
		Kind: "ComponentStatus", Group: "", Version: "v1", Plural: "componentstatuses",
		Namespaced: false, APIPath: "/api",
		SkipStatusSubresource: true,
	},

	"podtemplate": {
		Kind: "PodTemplate", Group: "", Version: "v1", Plural: "podtemplates",
		Namespaced: true, APIPath: "/api",
		Statusless: true, SkipStatusSubresource: true,
	},

	// ── apps/v1 ───────────────────────────────────────────────────────────────

	"deployment": {
		Kind: "Deployment", Group: "apps", Version: "v1", Plural: "deployments",
		Namespaced: true, APIPath: "/apis",
		IsChild: true, InrunInternal: true,
		Shorthands: []string{"deploy", "dep"},
		HookKey:    "deployments",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.DeploymentTemplateSource { return t.Deployments })
		},
	},

	"statefulset": {
		Kind: "StatefulSet", Group: "apps", Version: "v1", Plural: "statefulsets",
		Namespaced: true, APIPath: "/apis",
		HookKey:    "statefulSets",
		Shorthands: []string{"sts"},
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.StatefulSetTemplateSource { return t.StatefulSets })
		},
	},

	"daemonset": {
		Kind: "DaemonSet", Group: "apps", Version: "v1", Plural: "daemonsets",
		Namespaced: true, APIPath: "/apis",
		Shorthands: []string{"ds"},
	},

	"replicaset": {
		Kind: "ReplicaSet", Group: "apps", Version: "v1", Plural: "replicasets",
		Namespaced: true, APIPath: "/apis",
		Shorthands: []string{"rs"},
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ReplicaSetTemplateSource { return t.ReplicaSets })
		},
	},

	// ── batch/v1 ──────────────────────────────────────────────────────────────

	"job": {
		Kind: "Job", Group: "batch", Version: "v1", Plural: "jobs",
		Namespaced: true, APIPath: "/apis",
		SkipStatusSubresource: true, IsChild: true,
		HookKey: "jobs",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.JobTemplateSource { return t.Jobs })
		},
	},

	"cronjob": {
		Kind: "CronJob", Group: "batch", Version: "v1", Plural: "cronjobs",
		Namespaced: true, APIPath: "/apis",
		SkipStatusSubresource: true, IsChild: true,
		Shorthands: []string{"cj"},
		HookKey:    "cronJobs",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.CronJobTemplateSource { return t.CronJobs })
		},
	},

	// ── networking.k8s.io/v1 ─────────────────────────────────────────────────

	"ingress": {
		Kind: "Ingress", Group: "networking.k8s.io", Version: "v1", Plural: "ingresses",
		Namespaced: true, APIPath: "/apis",
		InrunInternal: true, IsChild: true,
		Shorthands: []string{"ing"},
		HookKey:    "ingresses",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.IngressTemplateSource { return t.Ingresses })
		},
	},

	"networkpolicy": {
		Kind: "NetworkPolicy", Group: "networking.k8s.io", Version: "v1", Plural: "networkpolicies",
		Namespaced: true, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"np"},
		HookKey:    "networkPolicies",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.NetworkPolicyTemplateSource { return t.NetworkPolicies })
		},
	},

	"ingressclass": {
		Kind: "IngressClass", Group: "networking.k8s.io", Version: "v1", Plural: "ingressclasses",
		Namespaced: false, APIPath: "/apis",
	},

	// ── autoscaling/v2 ────────────────────────────────────────────────────────

	"horizontalpodautoscaler": {
		Kind: "HorizontalPodAutoscaler", Group: "autoscaling", Version: "v2", Plural: "horizontalpodautoscalers",
		Namespaced: true, APIPath: "/apis",
		InrunInternal: true, IsChild: true,
		Shorthands: []string{"hpa"},
		HookKey:    "hpa",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.HPATemplateSource { return t.HorizontalPodAutoscalers })
		},
	},

	// ── rbac.authorization.k8s.io/v1 ─────────────────────────────────────────

	"role": {
		Kind: "Role", Group: "rbac.authorization.k8s.io", Version: "v1", Plural: "roles",
		Namespaced: true, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true, IsChild: true,
		HookKey: "roles",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.RoleTemplateSource { return t.Roles })
		},
	},

	"rolebinding": {
		Kind: "RoleBinding", Group: "rbac.authorization.k8s.io", Version: "v1", Plural: "rolebindings",
		Namespaced: true, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"rb"},
		HookKey:    "roleBindings",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.RoleBindingTemplateSource { return t.RoleBindings })
		},
	},

	"clusterrole": {
		Kind: "ClusterRole", Group: "rbac.authorization.k8s.io", Version: "v1", Plural: "clusterroles",
		Namespaced: false, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"cr"},
		HookKey:    "clusterRoles",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ClusterRoleTemplateSource { return t.ClusterRoles })
		},
	},

	"clusterrolebinding": {
		Kind: "ClusterRoleBinding", Group: "rbac.authorization.k8s.io", Version: "v1", Plural: "clusterrolebindings",
		Namespaced: false, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"crb"},
		HookKey:    "clusterRoleBindings",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.ClusterRoleBindingTemplateSource {
				return t.ClusterRoleBindings
			})
		},
	},

	// ── policy/v1 ─────────────────────────────────────────────────────────────

	"poddisruptionbudget": {
		Kind: "PodDisruptionBudget", Group: "policy", Version: "v1", Plural: "poddisruptionbudgets",
		Namespaced: true, APIPath: "/apis",
		SkipObservedGeneration: true, InrunInternal: true, IsChild: true,
		Shorthands: []string{"pdb"},
		HookKey:    "pdb",
		Detect: func(crd types.CRDEntry) bool {
			return detectAny(crd, func(t *types.HookTemplates) []types.PDBTemplateSource { return t.PodDisruptionBudgets })
		},
	},

	// ── storage.k8s.io/v1 ────────────────────────────────────────────────────

	"storageclass": {
		Kind: "StorageClass", Group: "storage.k8s.io", Version: "v1", Plural: "storageclasses",
		Namespaced: false, APIPath: "/apis",
		Shorthands: []string{"sc"},
	},

	"volumeattachment": {
		Kind: "VolumeAttachment", Group: "storage.k8s.io", Version: "v1", Plural: "volumeattachments",
		Namespaced: false, APIPath: "/apis",
		SkipObservedGeneration: true,
	},

	// ── apiextensions.k8s.io/v1 ──────────────────────────────────────────────

	"customresourcedefinition": {
		Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io", Version: "v1", Plural: "customresourcedefinitions",
		Namespaced: false, APIPath: "/apis",
		SkipObservedGeneration: true, InrunInternal: true,
		Shorthands: []string{"crd"},
	},

	// ── apiregistration.k8s.io/v1 ────────────────────────────────────────────

	"apiservice": {
		Kind: "APIService", Group: "apiregistration.k8s.io", Version: "v1", Plural: "apiservices",
		Namespaced: false, APIPath: "/apis",
		SkipObservedGeneration: true,
	},

	// ── admissionregistration.k8s.io/v1 ──────────────────────────────────────

	"mutatingwebhookconfiguration": {
		Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io", Version: "v1", Plural: "mutatingwebhookconfigurations",
		Namespaced: false, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true,
	},

	"validatingwebhookconfiguration": {
		Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io", Version: "v1", Plural: "validatingwebhookconfigurations",
		Namespaced: false, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true, InrunInternal: true,
	},

	// ── scheduling.k8s.io/v1 ─────────────────────────────────────────────────

	"priorityclass": {
		Kind: "PriorityClass", Group: "scheduling.k8s.io", Version: "v1", Plural: "priorityclasses",
		Namespaced: false, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true,
	},

	// ── events.k8s.io/v1 ─────────────────────────────────────────────────────
	// Internal key avoids clashing with core/v1 Event.

	"event_events": {
		Kind: "Event", Group: "events.k8s.io", Version: "v1", Plural: "events",
		Namespaced: true, APIPath: "/apis",
		Statusless: true, SkipStatusSubresource: true,
	},

	// ── discovery.k8s.io/v1 ──────────────────────────────────────────────────

	"endpointslice": {
		Kind: "EndpointSlice", Group: "discovery.k8s.io", Version: "v1", Plural: "endpointslices",
		Namespaced: true, APIPath: "/apis",
		SkipObservedGeneration: true,
		Shorthands:             []string{"ep"},
	},

	// ── coordination.k8s.io/v1 ───────────────────────────────────────────────

	"lease": {
		Kind: "Lease", Group: "coordination.k8s.io", Version: "v1", Plural: "leases",
		Namespaced: true, APIPath: "/apis",
		SkipObservedGeneration: true, InrunInternal: true,
	},
}

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
