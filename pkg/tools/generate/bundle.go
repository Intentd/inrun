package generate

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// RenderBundle assembles a complete installation bundle:
// Namespace (once) → ServiceAccounts → ClusterRoles → ClusterRoleBindings → ConfigMap.
// expandedYAML must be the output of blueprint.Blueprint.SerializeExpanded() —
// fully resolved, no OCI imports remaining. The ConfigMap embeds this content
// so the runtime never needs to do OCI pulls at startup.
//
// runtimeRules are bound to the inrun ClusterRole.
// gatewayRules are bound to the inrun-gateway ClusterRole.
// opts controls which components are included in the output.
func RenderBundle(
	runtimeRules, gatewayRules []rbacv1.PolicyRule,
	expandedYAML []byte,
	namespace, workloadNamespace string,
	opts BundleOptions,
) (string, error) {

	nsBytes, err := renderNamespace(namespace)
	if err != nil {
		return "", fmt.Errorf("render namespace: %w", err)
	}

	workloadNsBytes, err := renderNamespace(workloadNamespace)
	if err != nil {
		return "", fmt.Errorf("render task namespace: %w", err)
	}

	rbacBytes, err := renderRBAC(runtimeRules, gatewayRules, namespace, opts)
	if err != nil {
		return "", fmt.Errorf("render rbac: %w", err)
	}

	// Build YAML docs with proper separators
	var docs []string

	docs = append(docs, string(nsBytes))

	if workloadNamespace != "" {
		docs = append(docs, string(workloadNsBytes))
	}

	// renderRBAC prefixes each object with its own ---; strip the leading one
	// so the assembler loop below adds it uniformly alongside the other docs.
	docs = append(docs, strings.TrimPrefix(string(rbacBytes), "---\n"))

	// Include ConfigMap only when at least one process component is included.
	if opts.IncludeRuntime || opts.IncludeGateway {
		cmBytes, err := renderConfigMapBytes(expandedYAML, namespace)
		if err != nil {
			return "", fmt.Errorf("render configmap: %w", err)
		}
		docs = append(docs, string(cmBytes))
	}

	// Join all docs: each gets a leading --- from here.
	out := ""
	for i, d := range docs {
		out += "---\n" + d + "\n"
		if i < len(docs)-1 {
			out += "\n"
		}
	}

	return out, nil
}
