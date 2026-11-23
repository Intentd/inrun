package e2e

import (
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

// resolveGatewayEnabled inspects the blueprint file and returns true if the
// gateway block is present. This allows Helm installation to automatically
// enable the gateway chart when required by the blueprint.
func resolveGatewayEnabled(blueprintFile string) (bool, error) {
	var raw struct {
		Gateway *types.GatewayConfig `yaml:"gateway,omitempty"`
	}

	data, err := readLocal(blueprintFile)
	if err != nil {
		return false, err
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	return raw.Gateway != nil, nil
}
