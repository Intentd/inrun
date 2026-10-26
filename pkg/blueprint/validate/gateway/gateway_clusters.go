package gateway

import (
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
)

// validateGatewayClusters checks gateway.clusters entries and validates every
// static serve.cluster / target.cluster reference against the registered names.
// Template expressions are parse-checked only — name resolution is deferred to
// apply time.
func (c *checker) validateGatewayClusters() error {
	if !c.Blueprint.IsGatewayEnabled() || !c.Blueprint.Gateway.HasClusters() {
		return nil
	}

	if err := validateClusterEntries(c.Blueprint.Gateway, &c.Blueprint.Warnings); err != nil {
		return err
	}

	return validateServeClusterRefs(c)
}

// validateClusterEntries checks that each gateway.clusters entry is structurally valid:
// endpoint required, exactly one credential form declared, required fields present.
func validateClusterEntries(g *types.GatewayConfig, warnings *types.Warnings) error {
	if !g.HasClusters() {
		return nil
	}

	for name, cfg := range g.Clusters.Entries {
		prefix := fmt.Sprintf("gateway.clusters.%s", name)

		if cfg.Endpoint == "" {
			return fmt.Errorf("%s %s: endpoint is required", failureMark(), prefix)
		}

		if !cfg.HasCredentials() {
			return fmt.Errorf("%s %s: a credential form is required — declare secretRef (kubeconfig) or tokenRef + caRef (bearer token)", failureMark(), prefix)
		}

		// Both forms declared — exclusive.
		if cfg.HasSecretRef() && (cfg.HasTokenRef() || cfg.HasCARef()) {
			return fmt.Errorf("%s %s: secretRef and tokenRef/caRef are mutually exclusive — declare one credential form", failureMark(), prefix)
		}

		if cfg.HasSecretRef() {
			if err := validateSecretRefWithBlueprintWarning(cfg.SecretRef, prefix+".secretRef", warnings); err != nil {
				return err
			}
			if cfg.Insecure {
				return fmt.Errorf("%s %s: insecure is only valid with tokenRef — kubeconfig manages TLS settings internally", failureMark(), prefix)
			}
		}

		if cfg.HasTokenRef() || cfg.HasCARef() {
			if !cfg.HasTokenRef() {
				return fmt.Errorf("%s %s: caRef requires tokenRef — both must be declared together", failureMark(), prefix)
			}
			if !cfg.HasCARef() && !cfg.Insecure {
				return fmt.Errorf("%s %s: tokenRef requires caRef unless insecure: true is set", failureMark(), prefix)
			}
			if err := validateSecretRefWithBlueprintWarning(cfg.TokenRef, prefix+".tokenRef", warnings); err != nil {
				return err
			}
			if cfg.HasCARef() {
				if err := validateSecretRefWithBlueprintWarning(cfg.CARef, prefix+".caRef", warnings); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateServeClusterRefs checks serve.cluster and target.cluster values
// across all CRD entries. Static names must exist in gateway.clusters.
// Template expressions are validated with the full user-defined funcMap.
func validateServeClusterRefs(c *checker) error {
	for crdName, entry := range c.Blueprint.EnabledCRDs() {
		if entry.Serve == nil {
			continue
		}

		for i, cluster := range entry.Serve.Clusters {
			if err := validateClusterRef(
				c.Blueprint,
				crdName,
				cluster,
				fmt.Sprintf("spec.crds.%s.serve.clusters[%d]", crdName, i),
			); err != nil {
				return err
			}
		}

		for targetName, target := range entry.Serve.Target.Entries {
			for i, cluster := range target.Clusters {
				path := fmt.Sprintf("spec.crds.%s.serve.target.%s.clusters[%d]", crdName, targetName, i)
				if err := validateClusterRef(c.Blueprint, crdName, cluster, path); err != nil {
					return err
				}
				// target.clusters entries must also appear in serve.clusters.
				if !entry.Serve.ClusterAllowed(cluster) && !isTemplate(cluster) {
					return fmt.Errorf("%s %s: cluster %q not declared in serve.clusters", failureMark(), path, cluster)
				}
			}
		}
	}
	return nil
}

// validateClusterRef validates one cluster field value at the given path.
// Template expressions are validated with the full user-defined funcMap;
// static names must be registered.
func validateClusterRef(bp *blueprint.Blueprint, crdName, value, path string) error {
	if value == "" {
		return nil
	}

	funcMap := buildFuncMapForValidation(bp.Functions)
	if isTemplate(value) {
		if err := validateTemplate("serve.cluster", crdName, "cluster", path, value, funcMap); err != nil {
			return err
		}
		return nil
	}

	if _, ok := bp.Gateway.Cluster(value); !ok {
		return fmt.Errorf("%s %s: cluster %q is not defined in gateway.clusters", failureMark(), path, value)
	}
	return nil
}
