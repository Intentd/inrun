package types

// CrossNamespaceChecker is implemented by any resource template that copies
// itself across namespaces using fromNamespace / toNamespaces.
//
// These resources require a live API server to read the source object, so they
// cannot be executed during simulation. FilterSimulatable uses this interface to
// strip them from the hook templates before the fake reconciler runs, and returns
// human-readable skip notices so the simulate output explains what was omitted.
//
// Any future resource type that adds fromNamespace / toNamespaces fields must:
//  1. Implement CrossNamespaceChecker on its *TemplateSource type.
//  2. Add its slice to FilterSimulatable and the blueprint validator.
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

// filterItems removes cross-namespace copy resources from src and appends a
// skip notice for each one to skipped.
func filterItems[T CrossNamespaceChecker](src []T, skipped *[]string) []T {
	var out []T
	for _, item := range src {
		if item.IsCrossNamespaceCopy() {
			*skipped = append(*skipped, item.GetKind()+"/"+item.GetName()+": cross-namespace copy skipped in simulate — requires a live cluster")
		} else {
			out = append(out, item)
		}
	}
	return out
}

// FilterSimulatable returns a copy of h with all cross-namespace copy resources
// removed, along with a notice for each skipped resource.
//
// The simulate harness calls this on every HookTemplates phase (onCreate,
// onReconcile, onDelete) before building the fake reconciler. Resources that
// implement CrossNamespaceChecker and return true from IsCrossNamespaceCopy are
// omitted; the caller is responsible for printing the notices.
func FilterSimulatable(h HookTemplates) (filtered HookTemplates, skipped []string) {
	filtered = h
	filtered.Secrets = filterItems(h.Secrets, &skipped)
	filtered.ConfigMaps = filterItems(h.ConfigMaps, &skipped)
	filtered.NetworkPolicies = filterItems(h.NetworkPolicies, &skipped)
	filtered.ResourceQuotas = filterItems(h.ResourceQuotas, &skipped)
	filtered.LimitRanges = filterItems(h.LimitRanges, &skipped)
	return filtered, skipped
}
