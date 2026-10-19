package blueprint

import (
	"testing"
	"time"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── Deletion protection ───────────────────────────────────────────────────────

func TestIsDeletionProtectionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsDeletionProtectionEnabled())
	})

	t.Run("block declared, enabled omitted — default-on", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, bp.IsDeletionProtectionEnabled())
	})

	t.Run("block declared, enabled: false — disabled", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{Enabled: ptr.To(false)},
		}}
		assert.False(t, bp.IsDeletionProtectionEnabled())
	})
}

func TestDeletionProtectionFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{FailurePolicy: "Ignore"},
		}}
		assert.Equal(t, "Ignore", bp.DeletionProtectionFailurePolicy())
	})

	t.Run("falls back to hard default (Fail)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "Fail", bp.DeletionProtectionFailurePolicy())
	})
}

func TestDeletionProtectionCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{CleanupOnShutdown: true},
		}}
		assert.True(t, bp.DeletionProtectionCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.DeletionProtectionCleanupOnShutdown())
	})
}

func TestIsStrictModeEnabled(t *testing.T) {
	t.Run("no deletion protection block — false", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsStrictModeEnabled())
	})

	t.Run("strictMode: true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			DeletionProtection: &types.DeletionProtectionConfig{StrictMode: true},
		}}
		assert.True(t, bp.IsStrictModeEnabled())
	})
}

// ── Namespace protection ──────────────────────────────────────────────────────

func TestIsNamespaceProtectionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsNamespaceProtectionEnabled())
	})

	t.Run("block declared, enabled omitted — default-on", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{NamespaceProtection: &types.NamespaceProtectionConfig{}}}
		assert.True(t, bp.IsNamespaceProtectionEnabled())
	})

	t.Run("block declared, enabled: false — disabled", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{Enabled: ptr.To(false)},
		}}
		assert.False(t, bp.IsNamespaceProtectionEnabled())
	})
}

func TestNamespaceProtectionFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{FailurePolicy: "Ignore"},
		}}
		assert.Equal(t, "Ignore", bp.NamespaceProtectionFailurePolicy())
	})

	t.Run("falls back to hard default (Fail)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "Fail", bp.NamespaceProtectionFailurePolicy())
	})
}

func TestNamespaceProtectionCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			NamespaceProtection: &types.NamespaceProtectionConfig{CleanupOnShutdown: true},
		}}
		assert.True(t, bp.NamespaceProtectionCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.NamespaceProtectionCleanupOnShutdown())
	})
}

// ── Admission / conversion webhooks ───────────────────────────────────────────

func TestIsAdmissionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsAdmissionEnabled())
	})

	t.Run("declared but no explicit value — no default-on", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			Webhooks: &types.WebhooksConfig{Admission: &types.AdmissionWebhookToggle{}},
		}}
		assert.False(t, bp.IsAdmissionEnabled())
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{
			Webhooks: &types.WebhooksConfig{Admission: &types.AdmissionWebhookToggle{Enabled: ptr.To(true)}},
		}}
		assert.True(t, bp.IsAdmissionEnabled())
	})
}

func TestIsConversionEnabled(t *testing.T) {
	t.Run("block absent — disabled (no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsConversionEnabled())
	})

	t.Run("declared but no explicit value — no default-on", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Conversion: &types.ConversionConfig{}}}
		assert.False(t, bp.IsConversionEnabled())
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Conversion: &types.ConversionConfig{Enabled: ptr.To(true)}}}
		assert.True(t, bp.IsConversionEnabled())
	})
}

func TestWebhooksFailurePolicy(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Webhooks: &types.WebhooksConfig{FailurePolicy: "Fail"}}}
		assert.Equal(t, "Fail", bp.WebhooksFailurePolicy())
	})

	t.Run("falls back to hard default (Ignore)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "Ignore", bp.WebhooksFailurePolicy())
	})
}

func TestWebhookCleanupOnShutdown(t *testing.T) {
	t.Run("YAML true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Webhooks: &types.WebhooksConfig{CleanupOnShutdown: true}}}
		assert.True(t, bp.WebhookCleanupOnShutdown())
	})

	t.Run("block absent — falls back to default (false, no config)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.WebhookCleanupOnShutdown())
	})
}

func TestConversionWindow(t *testing.T) {
	t.Run("YAML value > 0 takes precedence", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Conversion: &types.ConversionConfig{ConversionWindow: 250}}}
		assert.Equal(t, 250, bp.ConversionWindow())
	})

	t.Run("YAML zero — falls back to hard default (100)", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{Conversion: &types.ConversionConfig{}}}
		assert.Equal(t, 100, bp.ConversionWindow())
	})

	t.Run("block absent — falls back to hard default (100)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, 100, bp.ConversionWindow())
	})
}

// ── Gateway config ────────────────────────────────────────────────────────────

func TestIsGatewayEnabled(t *testing.T) {
	t.Run("nil gateway — disabled", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsGatewayEnabled())
	})

	t.Run("gateway present but every field zero — still enabled", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{}}
		assert.True(t, bp.IsGatewayEnabled())
	})

	t.Run("gateway.enabled: true — enabled", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{Enabled: true}}
		assert.True(t, bp.IsGatewayEnabled())
	})

	t.Run("gateway.endpoint set, enabled omitted — enabled", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{Endpoint: "https://gateway.internal"}}
		assert.True(t, bp.IsGatewayEnabled())
	})

	t.Run("only gateway API declared, no enabled/endpoint — enabled", func(t *testing.T) {
		// The CI-only Gateway API client scenario: no runtime pairing, no
		// gateway.endpoint — just a CRUD surface for CI/Backstage/custom UIs
		// to call. The gateway: block still declares real config, so it
		// must count as enabled.
		bp := &Blueprint{Gateway: &types.GatewayConfig{
			API: &types.GatewayAPIConfig{Enabled: true},
		}}
		assert.True(t, bp.IsGatewayEnabled())
	})
}

func TestIsGatewayAPIEnabled(t *testing.T) {
	t.Run("nil gateway", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsGatewayAPIEnabled())
	})

	t.Run("gateway present, gateway API nil", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{}}
		assert.False(t, bp.IsGatewayAPIEnabled())
	})

	t.Run("gateway API present but not enabled", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{}}}
		assert.False(t, bp.IsGatewayAPIEnabled())
	})

	t.Run("gateway API enabled", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}}}
		assert.True(t, bp.IsGatewayAPIEnabled())
	})
}

func TestHasGatewayAPISecretRefs(t *testing.T) {
	t.Run("gateway API disabled — false", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.HasGatewayAPISecretRefs())
	})

	t.Run("enabled, no tokens use secretRef", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth:    types.APIAuth{Tokens: []types.APIToken{{Name: "ci", Token: "${INRUN_CI_TOKEN}"}}},
		}}}
		assert.False(t, bp.HasGatewayAPISecretRefs())
	})

	t.Run("enabled, one token uses secretRef", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth: types.APIAuth{Tokens: []types.APIToken{
				{Name: "ci", Token: "${INRUN_CI_TOKEN}"},
				{Name: "cc", SecretRef: &types.APISecretRef{Name: "inrun-apply-token", Key: "token"}},
			}},
		}}}
		assert.True(t, bp.HasGatewayAPISecretRefs())
	})
}

func TestHasServeEnabled(t *testing.T) {
	t.Run("gateway API disabled — false", func(t *testing.T) {
		bp := &Blueprint{enabledCRDs: map[string]types.CRDEntry{
			"Website": {Name: "Website", Serve: &types.ServeConfig{Enabled: true}},
		}}
		assert.False(t, bp.HasServeEnabled())
	})

	t.Run("gateway API enabled, no CRD opts into serve", func(t *testing.T) {
		bp := &Blueprint{
			Gateway:     &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}},
			enabledCRDs: map[string]types.CRDEntry{"Website": {Name: "Website"}},
		}
		assert.False(t, bp.HasServeEnabled())
	})

	t.Run("gateway API enabled, one CRD opts into serve", func(t *testing.T) {
		bp := &Blueprint{
			Gateway: &types.GatewayConfig{API: &types.GatewayAPIConfig{Enabled: true}},
			enabledCRDs: map[string]types.CRDEntry{
				"Website": {Name: "Website", Serve: &types.ServeConfig{Enabled: true}},
			},
		}
		assert.True(t, bp.HasServeEnabled())
	})
}

func TestIsStandaloneGateway(t *testing.T) {
	t.Run("nil gateway", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.IsStandaloneGateway())
	})

	t.Run("gateway present, standalone false", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{}}
		assert.False(t, bp.IsStandaloneGateway())
	})

	t.Run("standalone: true", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{Standalone: true}}
		assert.True(t, bp.IsStandaloneGateway())
	})
}

func TestGatewayEndpoint(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		bp := &Blueprint{Gateway: &types.GatewayConfig{Endpoint: "https://gateway.internal"}}
		assert.Equal(t, "https://gateway.internal", bp.GatewayEndpoint())
	})

	t.Run("no gateway, no config — empty", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "", bp.GatewayEndpoint())
	})
}

func TestClusterName(t *testing.T) {
	t.Run("metadata.clusterName set", func(t *testing.T) {
		bp := &Blueprint{}
		bp.metadata.ClusterName = "prod-us-east"
		assert.Equal(t, "prod-us-east", bp.ClusterName())
	})

	t.Run("no metadata, no config — defaults to runtime name", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "inrun-runtime", bp.ClusterName())
	})
}

// ── Gateway / certificate requirement ─────────────────────────────────────────

func TestNeedsCertificates(t *testing.T) {
	t.Run("nothing configured — false", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.NeedsCertificates())
	})

	t.Run("deletion protection enabled — true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, bp.NeedsCertificates())
	})

	t.Run("namespace protection enabled — true", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{NamespaceProtection: &types.NamespaceProtectionConfig{}}}
		assert.True(t, bp.NeedsCertificates())
	})
}

func TestNeedsGateway(t *testing.T) {
	t.Run("nothing configured — false", func(t *testing.T) {
		bp := &Blueprint{}
		assert.False(t, bp.NeedsGateway())
	})

	t.Run("deletion protection enabled — needs gateway for certs", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}}
		assert.True(t, bp.NeedsGateway())
	})
}

func TestIsHousekeeperEnabled(t *testing.T) {
	bp := &Blueprint{}
	assert.False(t, bp.IsHousekeeperEnabled()) // no config — hard default false
}

func TestHousekeeperSyncInterval(t *testing.T) {
	bp := &Blueprint{}
	assert.Equal(t, 30*time.Second, bp.HousekeeperSyncInterval()) // no config — hard default
}

// ── Certificate manager ───────────────────────────────────────────────────────

func TestCertAutoRotate(t *testing.T) {
	t.Run("YAML declared, autoRotate: false", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{CertManager: &types.CertManagerConfig{AutoRotate: ptr.To(false)}}}
		assert.False(t, bp.CertAutoRotate())
	})

	t.Run("block absent — falls back to hard default (true)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.True(t, bp.CertAutoRotate())
	})
}

func TestCertRotationThreshold(t *testing.T) {
	t.Run("YAML value parsed", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{CertManager: &types.CertManagerConfig{RotationThreshold: "7d"}}}
		assert.Equal(t, 7*24*time.Hour, bp.CertRotationThreshold())
	})

	t.Run("block absent — falls back to hard default (30d)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, 30*24*time.Hour, bp.CertRotationThreshold())
	})
}

func TestCertValidFor(t *testing.T) {
	t.Run("YAML value parsed", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{CertManager: &types.CertManagerConfig{ValidFor: "6mo"}}}
		assert.Equal(t, 6*30*24*time.Hour, bp.CertValidFor())
	})

	t.Run("block absent — falls back to hard default (1y)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, 365*24*time.Hour, bp.CertValidFor())
	})
}

func TestCertValidForStr(t *testing.T) {
	t.Run("YAML value takes precedence", func(t *testing.T) {
		bp := &Blueprint{Security: types.BlueprintSecurity{CertManager: &types.CertManagerConfig{ValidFor: "6mo"}}}
		assert.Equal(t, "6mo", bp.CertValidForStr())
	})

	t.Run("block absent — falls back to hard default (1y)", func(t *testing.T) {
		bp := &Blueprint{}
		assert.Equal(t, "1y", bp.CertValidForStr())
	})
}
