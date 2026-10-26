package gateway

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// validateGatewayTokens is the dispatcher for gateway.api.auth.tokens validation.
func (c *checker) validateGatewayTokens() error {
	if !c.Blueprint.IsGatewayEnabled() || !c.Blueprint.Gateway.HasAPI() || c.Blueprint.Gateway.API.Auth.Empty() {
		return nil
	}
	if err := c.validateGatewayTokensCommon(); err != nil {
		return err
	}
	if err := c.validateGatewayStaticTokens(); err != nil {
		return err
	}
	return c.validateGatewayOIDCTokens()
}

// gatewayAPIAuthTokens returns the token entries from gateway.api.auth.tokens.
// Callers must have already confirmed the gateway and API are enabled.
func (c *checker) gatewayAPIAuthTokens() []types.APIToken {
	return c.Blueprint.Gateway.API.Auth.Tokens
}

// validateGatewayTokensCommon checks for duplicate names and that each entry
// declares exactly one credential source.
func (c *checker) validateGatewayTokensCommon() error {
	seen := make(map[string]bool)
	var duplicates []string
	var errs []string

	for _, t := range c.gatewayAPIAuthTokens() {
		if seen[t.Name] {
			duplicates = append(duplicates, t.Name)
		}
		seen[t.Name] = true

		sourceCount := 0
		if strings.TrimSpace(t.Token) != "" {
			sourceCount++
		}
		if t.SecretRef != nil {
			sourceCount++
		}
		if t.IsOIDC() {
			sourceCount++
		}

		switch sourceCount {
		case 0:
			errs = append(errs, fmt.Sprintf(
				"  • gateway.api.auth.tokens[%q]: must set exactly one of: token, secretRef, githubOIDC, gitlabOIDC, vaultOIDC, oidc",
				t.Name))
		case 1:
			// ok
		default:
			errs = append(errs, fmt.Sprintf(
				"  • gateway.api.auth.tokens[%q]: only one of token, secretRef, githubOIDC, gitlabOIDC, vaultOIDC, oidc may be set",
				t.Name))
		}
	}

	if len(duplicates) > 0 {
		errs = append([]string{fmt.Sprintf(
			"  • gateway.api.auth.tokens: duplicate names: %s — each token name must be unique",
			red(strings.Join(duplicates, ", ")))}, errs...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s gateway.api.auth.tokens validation failed:\n%s", failureMark(), strings.Join(errs, "\n"))
	}
	return nil
}
