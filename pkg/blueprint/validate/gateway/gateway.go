package gateway

import (
	"fmt"
	"strings"
)

// validateGateway fails fast when gateway-dependent features are configured but
// no gatewayEndpoint is set. It collects every active feature that needs a
// gateway so the error tells the user everything at once.
// validateGateway fails fast when both standalone mode and an explicit endpoint are set.
// A simple "enabled: true" works
func (c *checker) validateGateway() error {
	if !c.Blueprint.NeedsGateway() {
		return nil
	}

	// If enabled, return early
	if c.Blueprint.IsGatewayEnabled() {
		return nil
	}

	// Conflict: only one of standalone or explicit endpoint can be used
	if c.Blueprint.IsStandaloneGateway() && c.Blueprint.GatewayEndpoint() != "" {
		return fmt.Errorf(
			"%s inrun: cannot enable both gateway.standalone and gateway.endpoint\n\n"+
				"Conflicting configuration:\n"+
				"  • gateway.standalone = true\n"+
				"  • gateway.endpoint = %q\n\n"+
				"Fix - set only one:\n"+
				"  • Standalone mode: set gateway.standalone = true and unset gateway.endpoint\n"+
				"  • Runtime Companion mode: set gateway.endpoint and set gateway.standalone = false (or omit it)",
			failureMark(), c.Blueprint.GatewayEndpoint(),
		)
	}

	// Standalone gateway mode – no endpoint needed
	if c.Blueprint.IsStandaloneGateway() {
		return nil
	}

	// External gateway endpoint provided – ok
	if c.Blueprint.GatewayEndpoint() != "" {
		return nil
	}

	// If we reach here, gateway is needed but no standalone or endpoint was set.
	var reasons []string
	if c.Blueprint.IsDeletionProtectionEnabled() {
		reasons = append(reasons, "  • security.deletionProtection")
	}
	if c.Blueprint.IsAdmissionEnabled() {
		reasons = append(reasons, "  • security.webhooks.admission")
	}
	if c.Blueprint.IsConversionEnabled() {
		reasons = append(reasons, "  • security.conversion")
	}
	if c.Blueprint.IsNamespaceProtectionEnabled() {
		reasons = append(reasons, "  • security.namespaceProtection")
	}

	return fmt.Errorf(
		"%s inrun: gateway endpoint required but not configured\n\n"+
			"Enabled features that need a gateway:\n%s\n\n"+
			"Fix - set gateway.endpoint, enable gateway.standalone, or set INRUN_GATEWAY_ENDPOINT.",
		failureMark(), strings.Join(reasons, "\n"),
	)
}
