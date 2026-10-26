package generate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	inrunRuntime = "inrun-runtime"
	inrunConsole = "inrun-console"
	inrunGateway = "inrun-gateway"
)

// RBAC generates RBAC resources with fine-grained control over which
// components are included. runtimeRules are bound to the inrun ClusterRole;
// gatewayRules are bound to the inrun-gateway ClusterRole.
func RBAC(runtimeRules, gatewayRules []rbacv1.PolicyRule, namespace, outputFile string, opts BundleOptions) ([]byte, error) {
	out, err := renderNamespaceAndRBAC(runtimeRules, gatewayRules, namespace, opts)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// renderNamespaceAndRBAC is the full standalone output: Namespace + ServiceAccounts + ClusterRole + ClusterRoleBinding.
func renderNamespaceAndRBAC(runtimeRules, gatewayRules []rbacv1.PolicyRule, namespace string, opts BundleOptions) ([]byte, error) {
	nsBytes, err := renderNamespace(namespace)
	if err != nil {
		return nil, err
	}
	rbacBytes, err := renderRBAC(runtimeRules, gatewayRules, namespace, opts)
	if err != nil {
		return nil, err
	}
	return prependNamespaceDoc(nsBytes, rbacBytes), nil
}

// renderNamespace marshals a Namespace object. Used by RBAC, ConfigMap, and Bundle
// to ensure the namespace exists before any namespaced resources are applied.
func renderNamespace(namespace string) ([]byte, error) {
	ns := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: labels.InrunBaseLabels(),
		},
	}
	return yaml.Marshal(ns)
}

// renderRBAC marshals ServiceAccounts, ClusterRoles, and ClusterRoleBindings only.
// The Namespace is intentionally excluded — callers prepend it via renderNamespace
// so that bundle assembly can include it exactly once.
//
// runtimeRules are bound to the "inrun" ClusterRole (runtime reconciler SA).
// gatewayRules are bound to the "inrun-gateway" ClusterRole (gateway SA).
// opts controls which components are emitted.
func renderRBAC(runtimeRules, gatewayRules []rbacv1.PolicyRule, namespace string, opts BundleOptions) ([]byte, error) {
	var objs []interface{}

	// ── ServiceAccounts ────────────────────────────────────────────────────────
	if opts.IncludeRuntime {
		objs = append(objs, corev1.ServiceAccount{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunRuntime, Namespace: namespace, Labels: labels.InrunResourceLabels()},
		})
	}
	if opts.IncludeGateway {
		objs = append(objs, corev1.ServiceAccount{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunGateway, Namespace: namespace, Labels: labels.InrunResourceLabels()},
		})
	}
	if opts.IncludeConsole {
		objs = append(objs, corev1.ServiceAccount{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunConsole, Namespace: namespace, Labels: labels.InrunResourceLabels()},
		})
	}

	// ── Runtime ClusterRole + ClusterRoleBinding ───────────────────────────────
	// Names are namespaced (inrun-<ns>) so multiple runtimes in different
	// namespaces don't overwrite each other's cluster-scoped RBAC objects.
	if opts.IncludeRuntime && len(runtimeRules) > 0 {
		inrunNS := inrunRuntime + "-" + namespace
		objs = append(objs,
			rbacv1.ClusterRole{
				TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"},
				ObjectMeta: metav1.ObjectMeta{Name: inrunNS, Labels: labels.InrunResourceLabels()},
				Rules:      runtimeRules,
			},
			rbacv1.ClusterRoleBinding{
				TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"},
				ObjectMeta: metav1.ObjectMeta{Name: inrunNS, Labels: labels.InrunResourceLabels()},
				RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: inrunNS},
				Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: inrunRuntime, Namespace: namespace}},
			},
		)
	}

	// ── Gateway ClusterRole + ClusterRoleBinding ───────────────────────────────
	if opts.IncludeGateway && len(gatewayRules) > 0 {
		inrunGatewayNS := inrunGateway + "-" + namespace
		objs = append(objs,
			rbacv1.ClusterRole{
				TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"},
				ObjectMeta: metav1.ObjectMeta{Name: inrunGatewayNS, Labels: labels.InrunResourceLabels()},
				Rules:      gatewayRules,
			},
			rbacv1.ClusterRoleBinding{
				TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"},
				ObjectMeta: metav1.ObjectMeta{Name: inrunGatewayNS, Labels: labels.InrunResourceLabels()},
				RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: inrunGatewayNS},
				Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: inrunGateway, Namespace: namespace}},
			},
		)
	}

	out := ""
	for i, obj := range objs {
		b, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("marshal rbac: %w", err)
		}
		out += "---\n" + string(b)
		if i < len(objs)-1 {
			out += "\n"
		}
	}
	return []byte(out), nil
}

// RBACForCluster generates a self-contained RBAC file to apply to a remote cluster.
// It produces a ServiceAccount, ClusterRole, and ClusterRoleBinding in saNamespace
// (typically "kube-system"), matching what inrun clusters bootstrap creates live.
// Apply this file to the remote cluster, not to the gateway cluster.
// saNamespace must already exist on the target cluster — "kube-system" always does.
func RBACForCluster(clusterName string, rules []rbacv1.PolicyRule, saNamespace string) ([]byte, error) {
	objs := []interface{}{
		corev1.ServiceAccount{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunGateway, Namespace: saNamespace, Labels: labels.InrunResourceLabels()},
		},
		rbacv1.ClusterRole{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunGateway, Labels: labels.InrunResourceLabels()},
			Rules:      rules,
		},
		rbacv1.ClusterRoleBinding{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"},
			ObjectMeta: metav1.ObjectMeta{Name: inrunGateway, Labels: labels.InrunResourceLabels()},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: inrunGateway},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: inrunGateway, Namespace: saNamespace}},
		},
	}

	out := ""
	for i, obj := range objs {
		b, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("marshal cluster rbac [%s]: %w", clusterName, err)
		}
		out += "---\n" + string(b)
		if i < len(objs)-1 {
			out += "\n"
		}
	}
	return []byte(out), nil
}

// prependNamespaceDoc places the Namespace as the first document in a multi-doc
// YAML stream, separated from the remaining docs by a blank line.
func prependNamespaceDoc(nsBytes, rest []byte) []byte {
	return []byte("---\n" + string(nsBytes) + "\n" + string(rest))
}
