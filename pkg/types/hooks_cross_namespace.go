package types

// CrossNamespaceChecker is implemented by any resource template that copies
// itself across namespaces using fromNamespace / toNamespaces. The blueprint
// validator uses it to check the pattern.
//
// Simulate runs these copies like any other template: the source is read
// with a plain client Get, which both backends serve — seed the source, and
// for envtest the target namespaces, with the simulate spec's setup.
//
// Any future resource type that adds fromNamespace / toNamespaces fields must
// implement CrossNamespaceChecker on its *TemplateSource type and be added
// to the blueprint validator.
//
// validate enforces that fromNamespace and toNamespaces must always be set
// together (see validate_cross_namespace.go), so checking either field alone
// is sufficient to detect the pattern here.
type CrossNamespaceChecker interface {
	IsCrossNamespaceCopy() bool
	GetName() string // already implemented on all *TemplateSource types via hooks_sleep.go
	GetKind() string
	GetFromNamespace() string
	GetToNamespaces() []string
}

// SecretTemplateSource: cross-namespace copy via fromSecret + fromNamespace → toNamespaces.
func (s SecretTemplateSource) IsCrossNamespaceCopy() bool {
	return s.FromNamespace != "" || len(s.ToNamespaces) > 0
}
func (s SecretTemplateSource) GetKind() string           { return "secrets" }
func (s SecretTemplateSource) GetFromNamespace() string  { return s.FromNamespace }
func (s SecretTemplateSource) GetToNamespaces() []string { return s.ToNamespaces }

// ConfigMapTemplateSource: cross-namespace copy via fromConfigMap + fromNamespace → toNamespaces.
func (s ConfigMapTemplateSource) IsCrossNamespaceCopy() bool {
	return s.FromNamespace != "" || len(s.ToNamespaces) > 0
}
func (s ConfigMapTemplateSource) GetKind() string           { return "configmaps" }
func (s ConfigMapTemplateSource) GetFromNamespace() string  { return s.FromNamespace }
func (s ConfigMapTemplateSource) GetToNamespaces() []string { return s.ToNamespaces }

// NetworkPolicyTemplateSource: cross-namespace copy via fromNetworkPolicy + fromNamespace → toNamespaces.
func (s NetworkPolicyTemplateSource) IsCrossNamespaceCopy() bool {
	return s.FromNamespace != "" || len(s.ToNamespaces) > 0
}
func (s NetworkPolicyTemplateSource) GetKind() string           { return "networkpolicies" }
func (s NetworkPolicyTemplateSource) GetFromNamespace() string  { return s.FromNamespace }
func (s NetworkPolicyTemplateSource) GetToNamespaces() []string { return s.ToNamespaces }

// ResourceQuotaTemplateSource: cross-namespace copy via fromResourceQuota + fromNamespace → toNamespaces.
func (s ResourceQuotaTemplateSource) IsCrossNamespaceCopy() bool {
	return s.FromNamespace != "" || len(s.ToNamespaces) > 0
}
func (s ResourceQuotaTemplateSource) GetKind() string           { return "resourcequotas" }
func (s ResourceQuotaTemplateSource) GetFromNamespace() string  { return s.FromNamespace }
func (s ResourceQuotaTemplateSource) GetToNamespaces() []string { return s.ToNamespaces }

// LimitRangeTemplateSource: cross-namespace copy via fromLimitRange + fromNamespace → toNamespaces.
func (s LimitRangeTemplateSource) IsCrossNamespaceCopy() bool {
	return s.FromNamespace != "" || len(s.ToNamespaces) > 0
}
func (s LimitRangeTemplateSource) GetKind() string           { return "limitranges" }
func (s LimitRangeTemplateSource) GetFromNamespace() string  { return s.FromNamespace }
func (s LimitRangeTemplateSource) GetToNamespaces() []string { return s.ToNamespaces }
