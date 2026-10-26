package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// ToGatewayClusterConfig converts a bootstrap result into a GatewayClusterConfig
// ready to paste into (or include from) gateway.clusters.
func (r *Result) ToGatewayClusterConfig() types.GatewayClusterConfig {
	cfg := types.GatewayClusterConfig{
		Endpoint: r.Endpoint,
		TokenRef: &types.APISecretRef{
			Name:      r.SecretName,
			Namespace: r.SecretNamespace,
			Key:       "token",
		},
	}
	if r.HasCA {
		cfg.CARef = &types.APISecretRef{
			Name:      r.SecretName,
			Namespace: r.SecretNamespace,
			Key:       "ca.crt",
		}
	}
	return cfg
}

// WriteClusterCredentials writes a clusters.yaml file in gateway.clusters format
// from a slice of bootstrap results. The file can be passed directly to
// `inrun clusters check --config` or included via `gateway.clusters.include:`.
func WriteClusterCredentials(path string, results []*Result) error {
	entries := make(map[string]types.GatewayClusterConfig, len(results))
	for _, r := range results {
		entries[r.Entry.Name] = r.ToGatewayClusterConfig()
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
