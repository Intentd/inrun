package catalog

import (
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── Deletion protection ───────────────────────────────────────────────────────

func TestIsDeletionProtectionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsDeletionProtectionEnabled())
	})

	t.Run("block declared, enabled omitted — default-on", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, cat.IsDeletionProtectionEnabled())
	})

	t.Run("block declared, enabled: false — disabled", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{Enabled: boolPtr(false)},
		}}
		assert.False(t, cat.IsDeletionProtectionEnabled())
	})
}

func TestDeletionProtectionServiceName(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{ServiceName: "custom-svc"},
		}}
		assert.Equal(t, "custom-svc", cat.DeletionProtectionServiceName())
	})

	t.Run("falls back to hard default when no config", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, inrunGate, cat.DeletionProtectionServiceName())
	})
}

func TestDeletionProtectionFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{FailurePolicy: "Ignore"},
		}}
		assert.Equal(t, "Ignore", cat.DeletionProtectionFailurePolicy())
	})

	t.Run("falls back to hard default (Fail)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "Fail", cat.DeletionProtectionFailurePolicy())
	})
}

func TestDeletionProtectionCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{CleanupOnShutdown: true},
		}}
		assert.True(t, cat.DeletionProtectionCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.DeletionProtectionCleanupOnShutdown())
	})
}

func TestIsStrictModeEnabled(t *testing.T) {
	t.Run("no deletion protection block — false", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsStrictModeEnabled())
	})

	t.Run("strictMode: true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{StrictMode: true},
		}}
		assert.True(t, cat.IsStrictModeEnabled())
	})
}

// ── Namespace protection ──────────────────────────────────────────────────────

func TestIsNamespaceProtectionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsNamespaceProtectionEnabled())
	})

	t.Run("block declared, enabled omitted — default-on", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{NamespaceProtection: &types.NamespaceProtectionConfig{}}}
		assert.True(t, cat.IsNamespaceProtectionEnabled())
	})

	t.Run("block declared, enabled: false — disabled", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{Enabled: boolPtr(false)},
		}}
		assert.False(t, cat.IsNamespaceProtectionEnabled())
	})
}

func TestNamespaceProtectionServiceName(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{ServiceName: "ns-svc"},
		}}
		assert.Equal(t, "ns-svc", cat.NamespaceProtectionServiceName())
	})

	t.Run("falls back to hard default when no config", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, inrunGate, cat.NamespaceProtectionServiceName())
	})
}

func TestNamespaceProtectionFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{FailurePolicy: "Ignore"},
		}}
		assert.Equal(t, "Ignore", cat.NamespaceProtectionFailurePolicy())
	})

	t.Run("falls back to hard default (Fail)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "Fail", cat.NamespaceProtectionFailurePolicy())
	})
}

func TestNamespaceProtectionCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{CleanupOnShutdown: true},
		}}
		assert.True(t, cat.NamespaceProtectionCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.NamespaceProtectionCleanupOnShutdown())
	})
}

// ── Admission / conversion webhooks ───────────────────────────────────────────

func TestIsAdmissionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsAdmissionEnabled())
	})

	t.Run("declared but no explicit value — no default-on", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			Webhooks: &types.WebhooksConfig{Admission: &types.AdmissionWebhookToggle{}},
		}}
		assert.False(t, cat.IsAdmissionEnabled())
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{
			Webhooks: &types.WebhooksConfig{Admission: &types.AdmissionWebhookToggle{Enabled: boolPtr(true)}},
		}}
		assert.True(t, cat.IsAdmissionEnabled())
	})
}

func TestIsConversionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsConversionEnabled())
	})

	t.Run("declared but no explicit value — no default-on", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Conversion: &types.ConversionConfig{}}}
		assert.False(t, cat.IsConversionEnabled())
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Conversion: &types.ConversionConfig{Enabled: boolPtr(true)}}}
		assert.True(t, cat.IsConversionEnabled())
	})
}

func TestWebhooksServiceName(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Webhooks: &types.WebhooksConfig{ServiceName: "wh-svc"}}}
		assert.Equal(t, "wh-svc", cat.WebhooksServiceName())
	})

	t.Run("falls back to hard default when no config", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, inrunGate, cat.WebhooksServiceName())
	})
}

func TestWebhooksFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Webhooks: &types.WebhooksConfig{FailurePolicy: "Fail"}}}
		assert.Equal(t, "Fail", cat.WebhooksFailurePolicy())
	})

	t.Run("falls back to hard default (Ignore)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "Ignore", cat.WebhooksFailurePolicy())
	})
}

func TestWebhookCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Webhooks: &types.WebhooksConfig{CleanupOnShutdown: true}}}
		assert.True(t, cat.WebhookCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.WebhookCleanupOnShutdown())
	})
}

func TestConversionWindow(t *testing.T) {
	t.Run("YAML value > 0 takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Conversion: &types.ConversionConfig{ConversionWindow: 250}}}
		assert.Equal(t, 250, cat.ConversionWindow())
	})

	t.Run("YAML zero — falls back to hard default (100)", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{Conversion: &types.ConversionConfig{}}}
		assert.Equal(t, 100, cat.ConversionWindow())
	})

	t.Run("block absent — falls back to hard default (100)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, 100, cat.ConversionWindow())
	})
}

// ── Gateway config ────────────────────────────────────────────────────────────

func TestIsGatewayEnabled(t *testing.T) {
	t.Run("nil gateway — disabled", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsGatewayEnabled())
	})

	t.Run("gateway present but every field zero — still enabled", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{}}
		assert.True(t, cat.IsGatewayEnabled())
	})

	t.Run("gateway.enabled: true — enabled", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{Enabled: true}}
		assert.True(t, cat.IsGatewayEnabled())
	})

	t.Run("gateway.endpoint set, enabled omitted — enabled", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{Endpoint: "https://gateway.internal"}}
		assert.True(t, cat.IsGatewayEnabled())
	})

	t.Run("only gateway API declared, no enabled/endpoint — enabled", func(t *testing.T) {
		// The CI-only Gateway API client scenario: no runtime pairing, no
		// gateway.endpoint — just a CRUD surface for CI/Backstage/custom UIs
		// to call. The gateway: block still declares real config, so it
		// must count as enabled.
		cat := &Catalog{Gateway: &types.GatewayConfig{
			API: &types.GatewayAPIConfig{Enabled: true},
		}}
		assert.True(t, cat.IsGatewayEnabled())
	})
}

func TestIsGatewayAPIEnabled(t *testing.T) {
	t.Run("nil gateway", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsGatewayAPIEnabled())
	})

	t.Run("gateway present, gateway API nil", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{}}
		assert.False(t, cat.IsGatewayAPIEnabled())
	})

	t.Run("gateway API present but not enabled", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{}}}
		assert.False(t, cat.IsGatewayAPIEnabled())
	})

	t.Run("gateway API enabled", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}}}
		assert.True(t, cat.IsGatewayAPIEnabled())
	})
}

func TestHasGatewayAPISecretRefs(t *testing.T) {
	t.Run("gateway API disabled — false", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.HasGatewayAPISecretRefs())
	})

	t.Run("enabled, no tokens use secretRef", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth:    types.APIAuth{Tokens: []types.APIToken{{Name: "ci", Token: "${INRUN_CI_TOKEN}"}}},
		}}}
		assert.False(t, cat.HasGatewayAPISecretRefs())
	})

	t.Run("enabled, one token uses secretRef", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth: types.APIAuth{Tokens: []types.APIToken{
				{Name: "ci", Token: "${INRUN_CI_TOKEN}"},
				{Name: "cc", SecretRef: &types.APISecretRef{Name: "inrun-apply-token", Key: "token"}},
			}},
		}}}
		assert.True(t, cat.HasGatewayAPISecretRefs())
	})
}

func TestHasServeEnabled(t *testing.T) {
	t.Run("gateway API disabled — false", func(t *testing.T) {
		cat := &Catalog{enabledCRDs: map[string]types.CRDEntry{
			"Website": {Name: "Website", Serve: &types.ServeConfig{Enabled: true}},
		}}
		assert.False(t, cat.HasServeEnabled())
	})

	t.Run("gateway API enabled, no CRD opts into serve", func(t *testing.T) {
		cat := &Catalog{
			Gateway:     &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}},
			enabledCRDs: map[string]types.CRDEntry{"Website": {Name: "Website"}},
		}
		assert.False(t, cat.HasServeEnabled())
	})

	t.Run("gateway API enabled, one CRD opts into serve", func(t *testing.T) {
		cat := &Catalog{
			Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}},
			enabledCRDs: map[string]types.CRDEntry{
				"Website": {Name: "Website", Serve: &types.ServeConfig{Enabled: true}},
			},
		}
		assert.True(t, cat.HasServeEnabled())
	})
}

func TestIsStandaloneGateway(t *testing.T) {
	t.Run("nil gateway", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.IsStandaloneGateway())
	})

	t.Run("gateway present, standalone false", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{}}
		assert.False(t, cat.IsStandaloneGateway())
	})

	t.Run("standalone: true", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{Standalone: true}}
		assert.True(t, cat.IsStandaloneGateway())
	})
}

func TestGatewayEndpoint(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Gateway: &types.GatewayConfig{Endpoint: "https://gateway.internal"}}
		assert.Equal(t, "https://gateway.internal", cat.GatewayEndpoint())
	})

	t.Run("no gateway, no config — empty", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "", cat.GatewayEndpoint())
	})
}

func TestClusterName(t *testing.T) {
	t.Run("metadata.clusterName set", func(t *testing.T) {
		cat := &Catalog{}
		cat.metadata.ClusterName = "prod-us-east"
		assert.Equal(t, "prod-us-east", cat.ClusterName())
	})

	t.Run("no metadata, no config — defaults to runtime name", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "inrun-runtime", cat.ClusterName())
	})
}

// ── Gateway / certificate requirement ─────────────────────────────────────────

func TestNeedsCertificates(t *testing.T) {
	t.Run("nothing configured — false", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.NeedsCertificates())
	})

	t.Run("deletion protection enabled — true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, cat.NeedsCertificates())
	})

	t.Run("namespace protection enabled — true", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{NamespaceProtection: &types.NamespaceProtectionConfig{}}}
		assert.True(t, cat.NeedsCertificates())
	})
}

func TestNeedsGateway(t *testing.T) {
	t.Run("nothing configured — false", func(t *testing.T) {
		cat := &Catalog{}
		assert.False(t, cat.NeedsGateway())
	})

	t.Run("deletion protection enabled — needs gateway for certs", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, cat.NeedsGateway())
	})
}

func TestIsHousekeeperEnabled(t *testing.T) {
	cat := &Catalog{}
	assert.False(t, cat.IsHousekeeperEnabled()) // no config — hard default false
}

func TestHousekeeperSyncInterval(t *testing.T) {
	cat := &Catalog{}
	assert.Equal(t, 30*time.Second, cat.HousekeeperSyncInterval()) // no config — hard default
}

// ── Certificate manager ───────────────────────────────────────────────────────

func TestCertAutoRotate(t *testing.T) {
	t.Run("YAML declared, autoRotate: false", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{CertManager: &types.CertManagerConfig{AutoRotate: boolPtr(false)}}}
		assert.False(t, cat.CertAutoRotate())
	})

	t.Run("block absent — falls back to hard default (true)", func(t *testing.T) {
		cat := &Catalog{}
		assert.True(t, cat.CertAutoRotate())
	})
}

func TestCertRotationThreshold(t *testing.T) {
	t.Run("YAML value parsed", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{CertManager: &types.CertManagerConfig{RotationThreshold: "7d"}}}
		assert.Equal(t, 7*24*time.Hour, cat.CertRotationThreshold())
	})

	t.Run("block absent — falls back to hard default (30d)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, 30*24*time.Hour, cat.CertRotationThreshold())
	})
}

func TestCertValidFor(t *testing.T) {
	t.Run("YAML value parsed", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{CertManager: &types.CertManagerConfig{ValidFor: "6mo"}}}
		assert.Equal(t, 6*30*24*time.Hour, cat.CertValidFor())
	})

	t.Run("block absent — falls back to hard default (1y)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, 365*24*time.Hour, cat.CertValidFor())
	})
}

func TestCertValidForStr(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		cat := &Catalog{Security: types.CatalogSecurity{CertManager: &types.CertManagerConfig{ValidFor: "6mo"}}}
		assert.Equal(t, "6mo", cat.CertValidForStr())
	})

	t.Run("block absent — falls back to hard default (1y)", func(t *testing.T) {
		cat := &Catalog{}
		assert.Equal(t, "1y", cat.CertValidForStr())
	})
}
