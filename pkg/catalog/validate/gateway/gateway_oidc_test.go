package gateway

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
)

func catalogWithOIDCToken(t types.APIToken) *checker {
	return newExec(&catalog.Catalog{
		Gateway: &types.GatewayConfig{
			Enabled: true,
			API: &types.GatewayAPIConfig{
				Enabled: true,
				Auth:    types.APIAuth{Tokens: []types.APIToken{t}},
			},
		},
	})
}

func TestValidateGatewayOIDC_GenericRequiresIssuer(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name: "ci",
		OIDC: &types.OIDCToken{
			// Issuer intentionally omitted
			Allow: map[string]string{"sub": "repo:myorg/app"},
		},
	})
	if err := e.validateGatewayOIDCTokens(); err == nil {
		t.Error("expected error: oidc.issuer required")
	} else if !strings.Contains(err.Error(), "oidc.issuer") {
		t.Errorf("error should mention oidc.issuer, got: %v", err)
	}
}

func TestValidateGatewayOIDC_GenericWithIssuer_OK(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name: "ci",
		OIDC: &types.OIDCToken{
			Issuer: "https://token.example.com",
			Allow:  map[string]string{"sub": "repo:myorg/app"},
		},
	})
	if err := e.validateGatewayOIDCTokens(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateGatewayOIDC_GitHub_OK(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name:       "gh-ci",
		GitHubOIDC: &types.GitHubOIDC{Allow: types.GitHubOIDCClaims{Repository: "myorg/app"}},
	})
	if err := e.validateGatewayOIDCTokens(); err != nil {
		t.Errorf("unexpected error for githubOIDC: %v", err)
	}
}

func TestValidateGatewayOIDC_GitLab_OK(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name:       "gl-ci",
		GitLabOIDC: &types.GitLabOIDC{Allow: types.GitLabOIDCClaims{NamespacePath: "mygroup/app"}},
	})
	if err := e.validateGatewayOIDCTokens(); err != nil {
		t.Errorf("unexpected error for gitlabOIDC: %v", err)
	}
}

func TestValidateGatewayOIDC_GitHub_EmptyAllow(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name:       "gh-ci",
		GitHubOIDC: &types.GitHubOIDC{}, // no allow fields
	})
	if err := e.validateGatewayOIDCTokens(); err == nil {
		t.Error("expected error: githubOIDC.allow is empty")
	} else if !strings.Contains(err.Error(), "githubOIDC.allow is empty") {
		t.Errorf("error should mention githubOIDC.allow is empty, got: %v", err)
	}
}

func TestValidateGatewayOIDC_GitLab_EmptyAllow(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name:       "gl-ci",
		GitLabOIDC: &types.GitLabOIDC{}, // no allow fields
	})
	if err := e.validateGatewayOIDCTokens(); err == nil {
		t.Error("expected error: gitlabOIDC.allow is empty")
	} else if !strings.Contains(err.Error(), "gitlabOIDC.allow is empty") {
		t.Errorf("error should mention gitlabOIDC.allow is empty, got: %v", err)
	}
}

func TestValidateGatewayOIDC_Vault_OK(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name: "vault-ci",
		VaultOIDC: &types.VaultOIDC{
			URL:   "https://vault.myorg.io",
			Allow: types.VaultOIDCClaims{EntityName: "ci-agent"},
		},
	})
	if err := e.validateGatewayOIDCTokens(); err != nil {
		t.Errorf("unexpected error for vaultOIDC: %v", err)
	}
}

func TestValidateGatewayOIDC_Vault_MissingURL(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name: "vault-ci",
		VaultOIDC: &types.VaultOIDC{
			// URL intentionally omitted
			Allow: types.VaultOIDCClaims{EntityName: "ci-agent"},
		},
	})
	if err := e.validateGatewayOIDCTokens(); err == nil {
		t.Error("expected error: vaultOIDC.url is required")
	} else if !strings.Contains(err.Error(), "vaultOIDC.url") {
		t.Errorf("error should mention vaultOIDC.url, got: %v", err)
	}
}

func TestValidateGatewayOIDC_Vault_EmptyAllow(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{
		Name:      "vault-ci",
		VaultOIDC: &types.VaultOIDC{URL: "https://vault.myorg.io"}, // no allow fields
	})
	if err := e.validateGatewayOIDCTokens(); err == nil {
		t.Error("expected error: vaultOIDC.allow is empty")
	} else if !strings.Contains(err.Error(), "vaultOIDC.allow is empty") {
		t.Errorf("error should mention vaultOIDC.allow is empty, got: %v", err)
	}
}

func TestValidateGatewayTokens_SourceExclusivity(t *testing.T) {
	// Both token and secretRef set — should fail.
	e := catalogWithOIDCToken(types.APIToken{
		Name:      "bad",
		Token:     "${MY_TOKEN}",
		SecretRef: &types.APISecretRef{Name: "s", Key: "k"},
	})
	if err := e.validateGatewayTokens(); err == nil {
		t.Error("expected error: only one source may be set")
	}
}

func TestValidateGatewayTokens_NoSource(t *testing.T) {
	e := catalogWithOIDCToken(types.APIToken{Name: "empty"})
	if err := e.validateGatewayTokens(); err == nil {
		t.Error("expected error: must set exactly one source")
	}
}
