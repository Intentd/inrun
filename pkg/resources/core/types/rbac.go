package types

// PolicyRuleSpec declares one RBAC policy rule.
// String values within slices support template expressions.
type PolicyRuleSpec struct {
	APIGroups     []string `yaml:"apiGroups,omitempty" json:"apiGroups,omitempty"`
	Resources     []string `yaml:"resources,omitempty" json:"resources,omitempty"`
	Verbs         []string `yaml:"verbs,omitempty" json:"verbs,omitempty"`
	ResourceNames []string `yaml:"resourceNames,omitempty" json:"resourceNames,omitempty"`
}

// SubjectSpec declares one RBAC subject for a RoleBinding.
// Name and Namespace support template expressions.
type SubjectSpec struct {
	Kind      string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// RoleRefSpec names the Role (or ClusterRole) being bound.
// Name supports template expressions. Kind defaults to "Role" on a
// RoleBinding and "ClusterRole" on a ClusterRoleBinding when omitted.
type RoleRefSpec struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"` // Role | ClusterRole
}
