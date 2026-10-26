package e2e

import (
	"fmt"

	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
)

func (r *Runner) installInrun() error {
	text := "..."

	// Console is never needed in e2e — disable it unconditionally.
	r.helmArgs = append(r.helmArgs, "--set", "console.enabled=false")

	gatewayEnabled, err := resolveGatewayEnabled(r.blueprintFile)
	if err != nil {
		return err
	}

	if gatewayEnabled {
		fmt.Printf("→ Gateway enabled...\n")
		r.helmArgs = append(r.helmArgs, "--set", "gateway.enabled=true")
		text = " with gateway..."
	}

	if r.noRuntime {
		r.helmArgs = append(r.helmArgs, "--set", "runtime.enabled=false")
		text = " gateway only..."
	}

	runtimeInstalled := cluster.RuntimeInstalled()
	gatewayInstalled := cluster.GatewayInstalled()

	if !runtimeInstalled && !gatewayInstalled {
		sp := startSpinner("Installing Inrun" + text)
		if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
			sp.Failure()
			return fmt.Errorf("helm install: %w", err)
		}
		sp.Success()
	} else {
		// Inrun is already running from a previous import.
		// 1. helm upgrade applies this import's values.
		// 2. SyncRuntime/SyncGateway restarts the pod to load the updated bundle ConfigMap.
		sp := startSpinner("Upgrading Inrun" + text)
		if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
			sp.Failure()
			return fmt.Errorf("helm upgrade: %w", err)
		}
		sp.Success()

		if !r.noRuntime {
			if err := cluster.SyncRuntime(); err != nil {
				return fmt.Errorf("syncing Inrun runtime: %w", err)
			}
		}

		if gatewayEnabled {
			if cluster.GatewayInstalled() {
				sp := startSpinner("→ Restarting gateway to load updated configuration...")
				if err := cluster.SyncGateway(); err != nil {
					sp.Failure()
					return fmt.Errorf("syncing Inrun gateway: %w", err)
				}
				sp.Success()
			} else {
				sp := startSpinner("Upgrading Inrun to enable gateway...")
				if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
					sp.Failure()
					return fmt.Errorf("helm upgrade (gateway): %w", err)
				}
				sp.Success()
			}
		}
	}

	if !r.noRuntime {
		status := cluster.CheckRuntimeHealth()
		if !status.Running {
			return fmt.Errorf("Inrun runtime not ready: %s", status.Reason)
		}
	}

	if gatewayEnabled {
		status := cluster.CheckGatewayHealth()
		if !status.Running {
			return fmt.Errorf("Inrun gateway not ready: %s", status.Reason)
		}
	}

	return nil
}

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
