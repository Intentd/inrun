// pkg/types/types_rbac.go
package types

import kwtypes "github.com/intentd/kubewright/core/types"

// ── Role / RoleBinding ────────────────────────────────────────────────────────

// PolicyRuleSpec is kwtypes.PolicyRuleSpec.
type PolicyRuleSpec = kwtypes.PolicyRuleSpec

// SubjectSpec is kwtypes.SubjectSpec.
type SubjectSpec = kwtypes.SubjectSpec

// RoleRefSpec is kwtypes.RoleRefSpec.
type RoleRefSpec = kwtypes.RoleRefSpec

// RoleTemplateSource declares one namespaced Role to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  roles:
//	    - name: "{{ .metadata.name }}-role"
//	      namespace: "{{ .metadata.name }}-ns"
//	      rules:
//	        - apiGroups: ["apps"]
//	          resources: ["deployments"]
//	          verbs: ["get", "list", "watch", "update", "patch"]
//	          resourceNames: ["{{ .metadata.name }}"]
type RoleTemplateSource struct {
	// Name — Role name.
	// Default when omitted: "{{ .metadata.name }}-role"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// Labels — applied to Role metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Rules — the permissions granted by this Role. Required: at least one rule.
	//
	//	rules:
	//	  - apiGroups: ["apps"]
	//	    resources: ["deployments"]
	//	    verbs: ["get", "list", "watch", "update", "patch"]
	//	    resourceNames: ["{{ .metadata.name }}"]
	Rules []PolicyRuleSpec `yaml:"rules,omitempty" json:"rules,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// RoleBindingTemplateSource declares one RoleBinding to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  roleBindings:
//	    - name: "{{ .metadata.name }}-rolebinding"
//	      namespace: "{{ .metadata.name }}-ns"
//	      roleRef:
//	        name: "{{ .metadata.name }}-role"
//	      subjects:
//	        - kind: ServiceAccount
//	          name: "{{ .metadata.name }}-sa"
//	          namespace: "{{ .metadata.name }}-ns"
type RoleBindingTemplateSource struct {
	// Name — RoleBinding name.
	// Default when omitted: "{{ .metadata.name }}-rolebinding"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// Labels — applied to RoleBinding metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// RoleRef — the Role or ClusterRole this binding grants. Required.
	// kind defaults to "Role" when omitted; set it to "ClusterRole" to bind a
	// cluster-scoped role instead.
	//
	//	roleRef:
	//	  name: "{{ .metadata.name }}-role"
	//	  kind: Role
	RoleRef RoleRefSpec `yaml:"roleRef,omitempty" json:"roleRef,omitempty"`

	// Subjects — the users, groups, or ServiceAccounts this binding grants
	// the role to. Required: at least one subject.
	//
	//	subjects:
	//	  - kind: ServiceAccount
	//	    name: "{{ .metadata.name }}-sa"
	//	    namespace: "{{ .metadata.namespace }}"
	Subjects []SubjectSpec `yaml:"subjects,omitempty" json:"subjects,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// ── ClusterRole / ClusterRoleBinding ─────────────────────────────────────────

// ClusterRoleTemplateSource declares one cluster-scoped ClusterRole to be managed by Inrun.
//
// ClusterRoles are cluster-scoped — no namespace field. Because Kubernetes cannot
// auto-GC cluster-scoped resources owned by namespace-scoped CRs, ownership is
// tracked via the inrun.dev/owner label. Declare cleanup in onDelete if needed.
//
// Example:
//
//	onCreate:
//	  clusterRoles:
//	    - name: "{{ .metadata.name }}-cluster-role"
//	      rules:
//	        - apiGroups: [""]
//	          resources: ["namespaces"]
//	          verbs: ["get", "list", "watch"]
type ClusterRoleTemplateSource struct {
	// Name — ClusterRole name.
	// Default when omitted: "{{ .metadata.name }}-cluster-role"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Labels — applied to ClusterRole metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Rules — the permissions granted by this ClusterRole. Required: at least one rule.
	//
	//	rules:
	//	  - apiGroups: [""]
	//	    resources: ["namespaces"]
	//	    verbs: ["get", "list", "watch"]
	Rules []PolicyRuleSpec `yaml:"rules,omitempty" json:"rules,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// ClusterRoleBindingTemplateSource declares one cluster-scoped ClusterRoleBinding to be managed by Inrun.
//
// ClusterRoleBindings are cluster-scoped — no namespace field.
// Ownership is tracked via the inrun.dev/owner label.
//
// Example:
//
//	onCreate:
//	  clusterRoleBindings:
//	    - name: "{{ .metadata.name }}-crb"
//	      roleRef:
//	        name: "{{ .metadata.name }}-cluster-role"
//	        kind: ClusterRole
//	      subjects:
//	        - kind: ServiceAccount
//	          name: "{{ .metadata.name }}-sa"
//	          namespace: "{{ .metadata.namespace }}"
type ClusterRoleBindingTemplateSource struct {
	// Name — ClusterRoleBinding name.
	// Default when omitted: "{{ .metadata.name }}-crb"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Labels — applied to ClusterRoleBinding metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// RoleRef — the ClusterRole this binding grants. Required.
	// kind defaults to "ClusterRole" when omitted (the only valid target for a
	// ClusterRoleBinding).
	//
	//	roleRef:
	//	  name: "{{ .metadata.name }}-cluster-role"
	RoleRef RoleRefSpec `yaml:"roleRef,omitempty" json:"roleRef,omitempty"`

	// Subjects — the users, groups, or ServiceAccounts this binding grants
	// the role to. Required: at least one subject.
	//
	//	subjects:
	//	  - kind: ServiceAccount
	//	    name: "{{ .metadata.name }}-sa"
	//	    namespace: "{{ .metadata.namespace }}"
	Subjects []SubjectSpec `yaml:"subjects,omitempty" json:"subjects,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
