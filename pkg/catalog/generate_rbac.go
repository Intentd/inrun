package catalog

import (
	"slices"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// The verb groups every generated rule is built from: readVerbs observe a
// resource, writeVerbs change it, and roleVerbs let a Role or ClusterRole grant
// permissions its creator does not hold (see rbacVerbsFor).
var (
	readVerbs  = []string{"get", "list", "watch"}
	writeVerbs = []string{"create", "update", "patch", "delete"}
	roleVerbs  = []string{"escalate", "bind"}
)

// Standard verbs for managed resources.
var defaultVerbs = concatVerbs(readVerbs, writeVerbs)

// watchVerbs is the minimal verb set for secondary watch resources.
// Watch entries only observe — they never write to watched resources.
var watchVerbs = concatVerbs(readVerbs)

// Writes reports whether rule lets its holder change objects: any write verb,
// or escalate/bind on roles.
func Writes(rule rbacv1.PolicyRule) bool {
	for _, v := range rule.Verbs {
		if slices.Contains(writeVerbs, v) || slices.Contains(roleVerbs, v) {
			return true
		}
	}
	return false
}

// concatVerbs returns a new slice holding groups in order.
func concatVerbs(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// rbacVerbsFor returns the appropriate verb set for a built-in resource.
//
// Roles and ClusterRoles require two extra verbs beyond standard CRUD:
//   - "escalate": allows creating/updating a Role or ClusterRole that grants
//     permissions the Inrun SA does not already hold. Without it Kubernetes
//     blocks any attempt to provision a role with a broader permission set.
//   - "bind": allows creating a RoleBinding or ClusterRoleBinding that
//     references a Role/ClusterRole whose permissions the SA doesn't hold.
//     Without it the binding creation is blocked even after the role exists.
//
// Both verbs are required whenever the operator provisions RBAC on behalf of
// tenant service accounts (e.g. via clusterRoles:/roles: in onCreate).
// They are absent from the generated bundle when no Roles or ClusterRoles are
// detected in the Catalog, preserving least-privilege for all other operators.
func rbacVerbsFor(group, plural string) []string {
	if group == "rbac.authorization.k8s.io" && (plural == "roles" || plural == "clusterroles") {
		return concatVerbs(defaultVerbs, roleVerbs)
	}
	return defaultVerbs
}

func (c *Catalog) GenerateRBACRules() []rbacv1.PolicyRule {
	var rules []rbacv1.PolicyRule

	// ───────────────────────────────────────────────
	// Base RBAC (always required)
	// ───────────────────────────────────────────────
	rules = append(rules,
		rbacv1.PolicyRule{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "create", "update"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch"},
		},
	)

	// ───────────────────────────────────────────────
	// Admission webhook RBAC (conditional)
	// ───────────────────────────────────────────────
	if c.NeedsCertificates() {
		webhookResources := c.WebhookResources()

		if len(webhookResources) > 0 {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{"admissionregistration.k8s.io"},
				Resources: webhookResources,
				Verbs:     defaultVerbs,
			})
		}

		// ───────────────────────────────────────────────
		// Needs permission to create and manage secret
		// ───────────────────────────────────────────────
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     defaultVerbs,
		})
	}

	// ───────────────────────────────────────────────
	// Deletion protection — namespace labeling
	// ───────────────────────────────────────────────
	// ensureNamespaceLabeled patches the Inrun namespace with deletion-protection
	// labels at startup so the admission webhook's ObjectSelector matches it.
	// Requires get (to confirm the namespace exists) and patch (to apply labels).
	if c.IsDeletionProtectionEnabled() {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "patch"},
		})

		// Custom CRs registered in the deletion-protection webhook need GET so
		// the gateway can read the instance and check its protection label/annotation.
		rules = append(rules, c.customResourceDeletionProtectionRBACRules()...)
	}

	// ───────────────────────────────────────────────
	// CRD RBAC (main + status)
	// ───────────────────────────────────────────────
	for _, crd := range c.Enabled() {
		if crd.APITypes.Group == "" || crd.APITypes.Plural == "" {
			if !crd.IsBuiltInType() {
				continue
			}
		}

		// Main resource
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{crd.APITypes.Group},
			Resources: []string{crd.APITypes.Plural},
			Verbs:     defaultVerbs,
		})

		// Status subresource
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{crd.APITypes.Group},
			Resources: []string{crd.APITypes.Plural + "/status"},
			Verbs:     []string{"get", "update", "patch"},
		})

		// CRD patching with CA bundle and watching for MODIFIED event by housekeeper
		if crd.EffectiveConversion() != nil && crd.UpdateCRDCaBundle() {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups:     []string{"apiextensions.k8s.io"},
				Resources:     []string{"customresourcedefinitions"},
				Verbs:         []string{"patch", "watch"},
				ResourceNames: []string{crd.APITypes.Plural + "." + crd.APITypes.Group},
			})
		}

		// Cross Declarations with secret references
		if crd.HasCrossSecretRef() {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			})
		}
	}

	// ───────────────────────────────────────────────
	// Typed‑mode RBAC (hooks or constructor)
	// ───────────────────────────────────────────────
	for _, crd := range c.Enabled() {

		// Resources hooks, constructor and remote reconcilers may create,
		// including per-target declarations.
		for _, r := range crd.AllManagedResources() {
			if gvr, ok := c.ResolveGVR(r); ok {
				rules = append(rules, rbacv1.PolicyRule{
					APIGroups: []string{gvr.Group},
					Resources: []string{gvr.Resource},
					Verbs:     defaultVerbs,
				})
			}
		}

		// Watch-entry resources — read-only
		for _, w := range crd.WatchEntries() {
			gvr, ok := c.ResolveGVR(w.ToManagedResource())
			if !ok {
				continue
			}
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{gvr.Group},
				Resources: []string{gvr.Resource},
				Verbs:     watchVerbs,
			})
		}
	}

	// ───────────────────────────────────────────────
	// Built-in resource RBAC — driven by builtInRegistry.
	// Any entry with a Detect function is a candidate; emit a rule when
	// at least one enabled CRD actually uses that resource.
	// ───────────────────────────────────────────────
	for _, b := range children.AllBuiltInKindDefs() {
		if b.Detect == nil {
			continue
		}
		if c.anyDetects(b.Detect) {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{b.Group},
				Resources: []string{b.Plural},
				Verbs:     rbacVerbsFor(b.Group, b.Plural),
			})
		}
	}

	// ───────────────────────────────────────────────
	// Custom resource RBAC — derived from onCreate/onReconcile custom: entries.
	// Built-in kinds are covered above; third-party CRDs (cert-manager, ArgoCD,
	// Crossplane, etc.) are not in the built-in registry and must be emitted here.
	// ───────────────────────────────────────────────
	rules = append(rules, c.customResourceRBACRules()...)

	return rules
}

// WebhookResources returns the list of admission webhook resources that Inrun
// needs to manage when webhooks/certificates are required.
//
// Rules:
//   - validatingwebhookconfigurations is required for deletion protection,
//     namespace protection, or any validation rules.
//   - mutatingwebhookconfigurations is required only when mutation rules exist.
//   - conversion webhooks are handled separately and do not require these resources.
func (c *Catalog) WebhookResources() []string {
	var resources []string

	// validatingwebhookconfigurations is needed for:
	// - deletion protection
	// - namespace protection
	// - validation rules (HasValidationRules)
	if c.IsDeletionProtectionEnabled() || c.IsNamespaceProtectionEnabled() || c.HasValidationRules() {
		resources = append(resources, "validatingwebhookconfigurations")
	}

	// mutatingwebhookconfigurations is only needed when mutation rules exist
	if c.HasMutationRules() {
		resources = append(resources, "mutatingwebhookconfigurations")
	}

	return resources
}

// secretNamesFrom partitions a slice of APISecretRef pointers into static names
// and a flag indicating whether any name is a template expression.
// Nil entries are skipped. Static and template names are collected independently
// so a single template does not suppress the scoped rule for the rest.
func secretNamesFrom(refs []*types.APISecretRef) (names []string, hasTemplate bool) {
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if isTemplate(ref.Name) {
			hasTemplate = true
		} else {
			names = append(names, ref.Name)
		}
	}
	return names, hasTemplate
}

// HasExternalSecretRefs returns true when any enabled CRD declares an external call
// with auth.secretRef. Used to gate automatic secrets-get RBAC generation.
func (c *Catalog) HasExternalSecretRefs() bool {
	names, hasTemplate := secretNamesFrom(c.collectExternalSecretRefs())
	return hasTemplate || len(names) > 0
}

// collectExternalSecretRefs gathers all auth.secretRef from external call
// declarations across all lifecycle blocks of every enabled CRD.
func (c *Catalog) collectExternalSecretRefs() []*types.APISecretRef {
	var refs []*types.APISecretRef
	for _, crd := range c.enabledCRDs {
		var calls []types.ExternalCallSpec
		if crd.Box().EffectiveOnReconcile() != nil {
			calls = append(calls, crd.Box().EffectiveOnReconcile().External...)
		}
		if crd.Box().EffectiveOnCreate() != nil {
			calls = append(calls, crd.Box().EffectiveOnCreate().External...)
		}
		calls = append(calls, crd.ReconcileExternal()...)
		if crd.EffectiveValidation() != nil {
			calls = append(calls, crd.EffectiveValidation().External...)
		}
		if crd.EffectiveMutation() != nil {
			calls = append(calls, crd.EffectiveMutation().External...)
		}
		for _, call := range calls {
			if call.Auth != nil {
				refs = append(refs, call.Auth.SecretRef)
			}
		}
	}
	return refs
}

// HasRemoteSecretRefs returns true when any enabled CRD declares a remote reconciler
// with auth.secretRef. Used to gate automatic secrets-get RBAC generation.
func (c *Catalog) HasRemoteSecretRefs() bool {
	names, hasTemplate := secretNamesFrom(c.collectRemoteSecretRefs())
	return hasTemplate || len(names) > 0
}

// collectRemoteSecretRefs gathers all auth.secretRef from remote reconciler
// declarations across every enabled CRD.
func (c *Catalog) collectRemoteSecretRefs() []*types.APISecretRef {
	var refs []*types.APISecretRef
	for _, crd := range c.enabledCRDs {
		r := crd.Box().Reconcile
		if r == nil || r.Remote == nil || r.Remote.Auth == nil {
			continue
		}
		refs = append(refs, r.Remote.Auth.SecretRef)
	}
	return refs
}

// collectGatewayAPISecretRefs gathers all secretRef from Gateway API token entries.
func (c *Catalog) collectGatewayAPISecretRefs() []*types.APISecretRef {
	if !c.IsGatewayAPIEnabled() {
		return nil
	}
	var refs []*types.APISecretRef
	for _, t := range c.Gateway.API.Auth.Tokens {
		refs = append(refs, t.SecretRef)
	}
	return refs
}

// GenerateRuntimeRBACRules returns the RBAC rules required by the runtime reconciler process.
// This is GenerateRBACRules() minus the NeedsCertificates() block (webhook/secrets),
// minus the IsDeletionProtectionEnabled() namespace block, and minus CRD CA-bundle patch rules.
func (c *Catalog) GenerateRuntimeRBACRules() []rbacv1.PolicyRule {
	var rules []rbacv1.PolicyRule

	// ───────────────────────────────────────────────
	// Base RBAC (always required)
	// ───────────────────────────────────────────────
	rules = append(rules,
		rbacv1.PolicyRule{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "create", "update"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch"},
		},
	)

	// ───────────────────────────────────────────────
	// External / remote auth.secretRef — secrets get
	// Static names get a scoped rule; template names get a separate unscoped
	// rule so one template does not prevent the others from being scoped.
	// ───────────────────────────────────────────────
	extNames, extTemplate := secretNamesFrom(c.collectExternalSecretRefs())
	remoteNames, remoteTemplate := secretNamesFrom(c.collectRemoteSecretRefs())
	if allStatic := append(extNames, remoteNames...); len(allStatic) > 0 {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: allStatic,
			Verbs:         []string{"get"},
		})
	}
	if extTemplate || remoteTemplate {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get"},
		})
	}

	// ───────────────────────────────────────────────
	// CRD RBAC (main + status, no CA bundle patch)
	// ───────────────────────────────────────────────
	for _, crd := range c.Enabled() {
		if crd.APITypes.Group == "" || crd.APITypes.Plural == "" {
			if !crd.IsBuiltInType() {
				continue
			}
		}

		// Main resource
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{crd.APITypes.Group},
			Resources: []string{crd.APITypes.Plural},
			Verbs:     defaultVerbs,
		})

		// Status subresource
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{crd.APITypes.Group},
			Resources: []string{crd.APITypes.Plural + "/status"},
			Verbs:     []string{"get", "update", "patch"},
		})
	}

	// ───────────────────────────────────────────────
	// Typed‑mode RBAC (hooks or constructor)
	// ───────────────────────────────────────────────
	for _, crd := range c.Enabled() {

		// Resources hooks, constructor and remote reconcilers may create,
		// including per-target declarations.
		for _, r := range crd.AllManagedResources() {
			if gvr, ok := c.ResolveGVR(r); ok {
				rules = append(rules, rbacv1.PolicyRule{
					APIGroups: []string{gvr.Group},
					Resources: []string{gvr.Resource},
					Verbs:     defaultVerbs,
				})
			}
		}

		// Watch-entry resources — read-only
		for _, w := range crd.WatchEntries() {
			gvr, ok := c.ResolveGVR(w.ToManagedResource())
			if !ok {
				continue
			}
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{gvr.Group},
				Resources: []string{gvr.Resource},
				Verbs:     watchVerbs,
			})
		}
	}

	// ───────────────────────────────────────────────
	// Built-in resource RBAC
	// ───────────────────────────────────────────────
	for _, b := range children.AllBuiltInKindDefs() {
		if b.Detect == nil {
			continue
		}
		if c.anyDetects(b.Detect) {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{b.Group},
				Resources: []string{b.Plural},
				Verbs:     rbacVerbsFor(b.Group, b.Plural),
			})
		}
	}

	// ───────────────────────────────────────────────
	// Custom resource RBAC
	// ───────────────────────────────────────────────
	rules = append(rules, c.customResourceRBACRules()...)

	return rules
}

// GenerateGatewayRBACRules returns the RBAC rules required by the gateway process
// (webhook server, certificate management, namespace labeling).
func (c *Catalog) GenerateGatewayRBACRules() []rbacv1.PolicyRule {
	if !c.IsGatewayEnabled() {
		return nil
	}

	var rules []rbacv1.PolicyRule

	// ───────────────────────────────────────────────
	// Admission webhook RBAC (conditional)
	// ───────────────────────────────────────────────
	if c.NeedsCertificates() {
		webhookResources := c.WebhookResources()

		if len(webhookResources) > 0 {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{"admissionregistration.k8s.io"},
				Resources: webhookResources,
				Verbs:     defaultVerbs,
			})
		}

		// Needs permission to create and manage secret
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     defaultVerbs,
		})
	}

	// ───────────────────────────────────────────────
	// Deletion protection — namespace labeling
	// ───────────────────────────────────────────────
	if c.IsDeletionProtectionEnabled() {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "patch"},
		})
	}

	// ───────────────────────────────────────────────
	// Gateway API — secretRef token bootstrap/rotation
	// ───────────────────────────────────────────────
	// get: read existing token; create: self-bootstrap when Secret is absent.
	// Uses the same annotation-based rotation as pkg/runners secrets_once.go.
	// get is scoped to ResourceNames when all names are static; create cannot
	// be (Kubernetes never authorizes create by resource name), so it gets its
	// own rule.
	gatewayNames, gatewayTemplate := secretNamesFrom(c.collectGatewayAPISecretRefs())
	if len(gatewayNames) > 0 {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: gatewayNames,
			Verbs:         []string{"get"},
		}, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"create"},
		})
	}
	if gatewayTemplate {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "create"},
		})
	}

	// ───────────────────────────────────────────────
	// Gateway API — CR create/update/delete/get/list
	// Only CRDs whose default routing is local (serve.cluster absent).
	// CRDs with a remote serve.cluster are handled in GenerateGatewayClusterRBACRules.
	// ───────────────────────────────────────────────
	if c.HasServeEnabled() {
		for _, crd := range c.Enabled() {
			if !crd.ServeEnabled() {
				continue
			}
			if crd.Serve.HasClusters() {
				continue // routed to remote clusters
			}
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{crd.APITypes.Group},
				Resources: []string{crd.APITypes.Plural},
				Verbs:     concatVerbs([]string{"get", "list"}, writeVerbs), // the gateway never watches
			})
			// Schema endpoint reads the CRD object to extract the OpenAPI spec.
			// Scoped to the exact CRD name — least-privilege get only.
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups:     []string{"apiextensions.k8s.io"},
				Resources:     []string{"customresourcedefinitions"},
				Verbs:         []string{"get"},
				ResourceNames: []string{crd.APITypes.Plural + "." + crd.APITypes.Group},
			})
		}
	}

	// ───────────────────────────────────────────────
	// CRD CA bundle patching (conversion webhooks)
	// ───────────────────────────────────────────────
	for _, crd := range c.Enabled() {
		if crd.EffectiveConversion() != nil && crd.UpdateCRDCaBundle() {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups:     []string{"apiextensions.k8s.io"},
				Resources:     []string{"customresourcedefinitions"},
				Verbs:         []string{"patch", "watch"},
				ResourceNames: []string{crd.APITypes.Plural + "." + crd.APITypes.Group},
			})
		}
	}

	return rules
}

// GenerateGatewayClusterRBACRules returns the RBAC rules the gateway needs on each
// registered remote cluster. The map key is the cluster name; the value is the
// minimal set of PolicyRules for CRDs statically routed to that cluster.
//
// The second return value lists the kinds of CRDs whose cluster routing uses a
// template expression — those rules are added to every cluster's entry. Callers
// should warn the user and ask them to remove entries for clusters that should not
// have access to template-routed CRDs.
//
// CRDs with no serve.cluster (local fallback) do not appear in the returned map.
// They are handled by GenerateGatewayRBACRules instead.
func (c *Catalog) GenerateGatewayClusterRBACRules() (map[string][]rbacv1.PolicyRule, []string) {
	// Return early if gateway is disabled, serve is disabled, or NO clusters are defined
	if !c.IsGatewayEnabled() || !c.HasServeEnabled() || c.GatewayClustersEmpty() {
		return nil, nil
	}

	clusters := c.GatewayClusters()
	// Initialize a slot for every registered cluster
	clusterRules := make(map[string][]rbacv1.PolicyRule, len(clusters))
	for name := range clusters {
		clusterRules[name] = nil
	}

	var templateKinds []string

	for _, crd := range c.ServeEnabledCRDs() {
		// Build rules for this specific CRD
		crdRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{crd.APITypes.Group},
				Resources: []string{crd.APITypes.Plural, crd.APITypes.Plural + "/status"},
				Verbs:     concatVerbs([]string{"get", "list"}, writeVerbs), // the gateway never watches
			},
			{
				APIGroups:     []string{"apiextensions.k8s.io"},
				Resources:     []string{"customresourcedefinitions"},
				Verbs:         []string{"get", "list"},
				ResourceNames: []string{crd.APITypes.Plural + "." + crd.APITypes.Group},
			},
		}

		staticTargets := map[string]bool{}
		hasTemplate := false

		// Check clusters defined at CRD level
		if crd.Serve != nil {
			for _, sc := range crd.Serve.Clusters {
				if isTemplate(sc) {
					hasTemplate = true
				} else {
					staticTargets[sc] = true
				}
			}

			// Check clusters defined at target level
			for _, entry := range crd.Serve.Target.Entries {
				for _, tc := range entry.TargetClusters() {
					if isTemplate(tc) {
						hasTemplate = true
					} else {
						staticTargets[tc] = true
					}
				}
			}
		}

		if hasTemplate {
			templateKinds = append(templateKinds, crd.APITypes.Kind)
			for name := range clusters {
				clusterRules[name] = append(clusterRules[name], crdRules...)
			}
		} else if len(staticTargets) > 0 {
			// Assign rules only to static targets
			for clusterName := range staticTargets {
				if _, ok := clusterRules[clusterName]; ok {
					clusterRules[clusterName] = append(clusterRules[clusterName], crdRules...)
				}
			}
		}
		// If no clusters and no targets (local only), skip - no RBAC needed
	}

	// Drop clusters that received no rules
	for name, rules := range clusterRules {
		if len(rules) == 0 {
			delete(clusterRules, name)
		}
	}

	return clusterRules, templateKinds
}

// ResolveGVR resolves a ManagedResource into a concrete GroupVersionResource.
func (c *Catalog) ResolveGVR(mr domain.ManagedResource) (schema.GroupVersionResource, bool) {
	return children.ResolveGVR(mr)
}

// ResolveGVK resolves a ManagedResource into a concrete GroupVersionKind.
func (c *Catalog) ResolveGVK(mr domain.ManagedResource) (schema.GroupVersionKind, bool) {
	return children.ResolveGVK(mr)
}

// GeneratePerCRDRBACRules returns the RBAC rules attributed to each enabled CRD.
// The map key is the CRD name. Excludes system-level rules (leases, events,
// secrets, namespaces, webhook configurations) — use GenerateRuntimeRBACRules /
// GenerateGatewayRBACRules for those.
func (c *Catalog) GeneratePerCRDRBACRules() map[string][]rbacv1.PolicyRule {
	result := make(map[string][]rbacv1.PolicyRule, c.Len())
	for name, crd := range c.Enabled() {
		result[name] = c.RBACRulesForCRD(crd)
	}
	return result
}

// RBACRulesForCRD returns the rules one CRD contributes: the CRD and its status,
// its managed resources, read-only watch entries, and the kinds its templates
// create. validate --full, the console and the generated bundle all read it.
func (c *Catalog) RBACRulesForCRD(crd types.CRDEntry) []rbacv1.PolicyRule {
	var rules []rbacv1.PolicyRule

	if crd.APITypes.Group != "" || crd.IsBuiltInType() {
		if crd.APITypes.Plural != "" {
			rules = append(rules,
				rbacv1.PolicyRule{
					APIGroups: []string{crd.APITypes.Group},
					Resources: []string{crd.APITypes.Plural},
					Verbs:     defaultVerbs,
				},
				rbacv1.PolicyRule{
					APIGroups: []string{crd.APITypes.Group},
					Resources: []string{crd.APITypes.Plural + "/status"},
					Verbs:     []string{"get", "update", "patch"},
				},
			)
		}
		if crd.EffectiveConversion() != nil && crd.UpdateCRDCaBundle() {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups:     []string{"apiextensions.k8s.io"},
				Resources:     []string{"customresourcedefinitions"},
				Verbs:         []string{"patch"},
				ResourceNames: []string{crd.APITypes.Plural + "." + crd.APITypes.Group},
			})
		}
	}

	// Resources hooks, constructor and remote reconcilers may create,
	// including per-target declarations.
	for _, r := range crd.AllManagedResources() {
		if gvr, ok := c.ResolveGVR(r); ok {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{gvr.Group},
				Resources: []string{gvr.Resource},
				Verbs:     defaultVerbs,
			})
		}
	}

	// Watch-entry resources — read-only
	for _, w := range crd.WatchEntries() {
		if gvr, ok := c.ResolveGVR(w.ToManagedResource()); ok {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{gvr.Group},
				Resources: []string{gvr.Resource},
				Verbs:     watchVerbs,
			})
		}
	}

	for _, b := range children.AllBuiltInKindDefs() {
		if b.Detect != nil && b.Detect(crd) {
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{b.Group},
				Resources: []string{b.Plural},
				Verbs:     defaultVerbs,
			})
		}
	}

	for _, rule := range customRBACRulesForCRD(crd) {
		rules = append(rules, rule)
	}

	return rules
}

// customResourceRBACRules collects RBAC rules for all third-party CRDs declared
// in onCreate/onReconcile custom: blocks across every enabled CRD.
// Built-in Kubernetes kinds are already covered by the builtInRegistry loop;
// this handles everything else (cert-manager, ArgoCD, Crossplane, etc.).
func (c *Catalog) customResourceRBACRules() []rbacv1.PolicyRule {
	var rules []rbacv1.PolicyRule
	seen := make(map[string]bool)
	for _, crd := range c.Enabled() {
		for _, rule := range customRBACRulesForCRD(crd) {
			key := strings.Join(rule.APIGroups, ",") + "/" + strings.Join(rule.Resources, ",")
			if !seen[key] {
				seen[key] = true
				rules = append(rules, rule)
			}
		}
	}
	return rules
}

// customRBACRulesForCRD derives RBAC rules from the custom: entries of one CRD's
// onCreate and onReconcile blocks. Each entry's apiVersion + kind is resolved into
// a group + plural via ParseGroupVersion and lowercase+s inference.
func customRBACRulesForCRD(crd types.CRDEntry) []rbacv1.PolicyRule {
	var entries []types.CustomResourceTemplateSource
	if crd.Box().EffectiveOnCreate() != nil {
		entries = append(entries, crd.Box().EffectiveOnCreate().CustomResource...)
	}
	if crd.Box().EffectiveOnReconcile() != nil {
		entries = append(entries, crd.Box().EffectiveOnReconcile().CustomResource...)
	}

	seen := make(map[string]bool)
	var rules []rbacv1.PolicyRule
	for _, entry := range entries {
		if entry.APIVersion == "" || entry.Kind == "" {
			continue
		}
		gv, err := schema.ParseGroupVersion(entry.APIVersion)
		if err != nil {
			continue
		}
		plural := strings.ToLower(entry.Kind) + "s"
		key := gv.Group + "/" + plural
		if seen[key] {
			continue
		}
		seen[key] = true
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{gv.Group},
			Resources: []string{plural},
			Verbs:     defaultVerbs,
		})
	}
	return rules
}

// customResourceDeletionProtectionRBACRules returns gateway RBAC rules granting
// GET on every custom child resource that the deletion-protection webhook
// intercepts. Mirrors customRBACRulesForCRD but uses get-only verbs — the
// gateway reads each object to check its protection label before allowing the DELETE.
func (c *Catalog) customResourceDeletionProtectionRBACRules() []rbacv1.PolicyRule {
	seen := make(map[string]bool)
	var rules []rbacv1.PolicyRule
	for _, crd := range c.Enabled() {
		for _, rule := range customRBACRulesForCRD(crd) {
			key := strings.Join(rule.APIGroups, ",") + "/" + strings.Join(rule.Resources, ",")
			if seen[key] {
				continue
			}
			seen[key] = true
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: rule.APIGroups,
				Resources: rule.Resources,
				Verbs:     []string{"get"},
			})
		}
	}
	return rules
}
