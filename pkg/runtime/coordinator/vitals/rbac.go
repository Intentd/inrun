package vitals

import (
	"fmt"
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/catalog"
	rbacv1 "k8s.io/api/rbac/v1"
)

type RBACInfo struct {
	Rules      []RBACRule `json:"rules"`
	Summary    string     `json:"summary"`
	TotalRules int        `json:"totalRules"`
}

type RBACRule struct {
	APIGroups []string `json:"apiGroups,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Verbs     []string `json:"verbs,omitempty"`
	// Human-readable description
	Description string `json:"description,omitempty"`
}

// toRBACInfo turns the rules the Catalog generates for one CRD
// (catalog.RBACRulesForCRD) into the console's view of them. The rules are
// not derived here, so the console shows what validate --full and the bundle do.
func toRBACInfo(policy []rbacv1.PolicyRule) RBACInfo {
	rules := make([]RBACRule, 0, len(policy))
	for _, p := range policy {
		rules = append(rules, RBACRule{
			APIGroups:   p.APIGroups,
			Resources:   p.Resources,
			Verbs:       p.Verbs,
			Description: describeRule(p),
		})
	}
	rules = deduplicateRules(rules)
	return RBACInfo{
		Rules:      rules,
		Summary:    generateSummary(rules),
		TotalRules: len(rules),
	}
}

// describeRule names what a rule allows: manage, read, or update status.
func describeRule(p rbacv1.PolicyRule) string {
	what := strings.Join(p.Resources, ", ")
	for _, r := range p.Resources {
		if strings.HasSuffix(r, "/status") {
			return "Update status of " + strings.TrimSuffix(r, "/status")
		}
	}
	if catalog.Writes(p) {
		return "Manage " + what
	}
	return "Read " + what
}

func generateSummary(rules []RBACRule) string {
	resourceCount := make(map[string]int)
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			resourceCount[resource]++
		}
	}

	if len(resourceCount) == 0 {
		return "No RBAC permissions required"
	}

	// Extract and sort keys
	keys := make([]string, 0, len(resourceCount))
	for resource := range resourceCount {
		keys = append(keys, resource)
	}
	sort.Strings(keys)

	// Build sorted output
	parts := make([]string, 0, len(keys))
	for _, resource := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", resourceCount[resource], resource))
	}

	return strings.Join(parts, ", ")
}

func deduplicateRules(rules []RBACRule) []RBACRule {
	seen := make(map[string]bool)
	result := []RBACRule{}

	for _, rule := range rules {
		key := fmt.Sprintf("%v|%v|%v", rule.APIGroups, rule.Resources, rule.Verbs)
		if !seen[key] {
			seen[key] = true
			result = append(result, rule)
		}
	}

	return result
}
