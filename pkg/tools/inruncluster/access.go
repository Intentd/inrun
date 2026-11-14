package inruncluster

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster/access"
	"github.com/intentd/tools/structured"
)

// Naming is what `inrun clusters access` calls the gateway's access: the
// inrun-gateway ServiceAccount on the target and inrun-<cluster> on the hub.
var Naming = access.Naming{
	ServiceAccount:  "inrun-gateway",
	HubSecretPrefix: "inrun",
	Labels: map[string]string{
		"app.kubernetes.io/managed-by": "inrun",
		"inrun.dev/role":               "gateway-cluster-access",
	},
	AnnotationPrefix: "inrun.dev",
}

// DefaultToken is the token the CLI grants when the config names none:
// short, which the gateway renews itself (see api.CredentialRefresher).
const DefaultToken = access.TokenShort

// GatewayRules are the ClusterRole rules the gateway needs on a target: full
// access to each served CRD and its status, grouped by API group.
func GatewayRules(bp *blueprint.Blueprint) []rbacv1.PolicyRule {
	type groupEntry struct{ resources []string }
	groups := map[string]*groupEntry{}
	verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}

	for _, crd := range bp.ServeEnabledCRDs() {
		gvr := crd.GVR()
		if groups[gvr.Group] == nil {
			groups[gvr.Group] = &groupEntry{}
		}
		groups[gvr.Group].resources = append(groups[gvr.Group].resources, gvr.Resource)
		groups[gvr.Group].resources = append(groups[gvr.Group].resources, gvr.Resource+"/status")
	}

	rules := make([]rbacv1.PolicyRule, 0, len(groups))
	for group, entry := range groups {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{group},
			Resources: structured.DedupStrings(entry.resources),
			Verbs:     verbs,
		})
	}
	return rules
}

// GatewayClusterConfig converts an access result into a GatewayClusterConfig
// ready to paste into (or include from) gateway.clusters.
func GatewayClusterConfig(r *access.Result) types.GatewayClusterConfig {
	cfg := types.GatewayClusterConfig{
		Endpoint: r.Endpoint,
		TokenRef: &types.KubernetesSecretRef{
			Name:      r.SecretName,
			Namespace: r.SecretNamespace,
			Key:       access.TokenKey,
		},
	}
	if r.HasCA {
		cfg.CARef = &types.KubernetesSecretRef{
			Name:      r.SecretName,
			Namespace: r.SecretNamespace,
			Key:       access.CAKey,
		}
	}
	return cfg
}

// WriteClusterCredentials writes a clusters.yaml file in gateway.clusters format
// from a slice of access results (revoked ones are left out). The file can be passed directly to
// `inrun clusters check --config` or included via `gateway.clusters.include:`.
func WriteClusterCredentials(path string, results []*access.Result) error {
	entries := make(map[string]types.GatewayClusterConfig, len(results))
	for _, r := range results {
		if !r.Revoked {
			entries[r.Name] = GatewayClusterConfig(r)
		}
	}

	out := struct {
		Clusters map[string]types.GatewayClusterConfig `yaml:"clusters"`
	}{Clusters: entries}

	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshalling cluster credentials: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// GatewayAccess sets opts up for the gateway's access: Inrun's Naming, the
// Secret format the gateway reads, and with a Blueprint, GatewayRules
// instead of each cluster's rules.
func GatewayAccess(bp *blueprint.Blueprint, opts access.Options) access.Options {
	opts.Naming = Naming
	opts.Format = access.SecretFormat{}
	if bp != nil {
		opts.Rules = GatewayRules(bp)
	}
	return opts
}
