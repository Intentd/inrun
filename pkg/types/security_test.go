// Tests for CatalogSecurity methods (security.go).
package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
)

// ── IsDeletionProtectionEnabled ───────────────────────────────────────────────

func TestIsDeletionProtectionEnabled_Nil(t *testing.T) {
	var s *types.CatalogSecurity
	assert.False(t, s.IsDeletionProtectionEnabled())
}

func TestIsDeletionProtectionEnabled_NilBlock(t *testing.T) {
	s := &types.CatalogSecurity{}
	assert.False(t, s.IsDeletionProtectionEnabled())
}

func TestIsDeletionProtectionEnabled_DeclaredNoEnabled(t *testing.T) {
	// Declared but no explicit enabled field = enabled by default
	s := &types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{}}
	assert.True(t, s.IsDeletionProtectionEnabled())
}

func TestIsDeletionProtectionEnabled_ExplicitFalse(t *testing.T) {
	f := false
	s := &types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{Enabled: &f}}
	assert.False(t, s.IsDeletionProtectionEnabled())
}

func TestIsDeletionProtectionEnabled_ExplicitTrue(t *testing.T) {
	tr := true
	s := &types.CatalogSecurity{DeletionProtection: &types.DeletionProtectionConfig{Enabled: &tr}}
	assert.True(t, s.IsDeletionProtectionEnabled())
}

// ── IsAdmissionEnabled ────────────────────────────────────────────────────────

func TestIsAdmissionEnabled_Nil(t *testing.T) {
	var s *types.CatalogSecurity
	assert.False(t, s.IsAdmissionEnabled())
}

func TestIsAdmissionEnabled_NoWebhooks(t *testing.T) {
	s := &types.CatalogSecurity{}
	assert.False(t, s.IsAdmissionEnabled())
}

func TestIsAdmissionEnabled_NoAdmission(t *testing.T) {
	s := &types.CatalogSecurity{Webhooks: &types.WebhooksConfig{}}
	assert.False(t, s.IsAdmissionEnabled())
}

func TestIsAdmissionEnabled_NilEnabledField(t *testing.T) {
	// Admission declared but Enabled not set → false (no default-on for webhooks)
	s := &types.CatalogSecurity{
		Webhooks: &types.WebhooksConfig{Admission: &types.AdmissionWebhookToggle{}},
	}
	assert.False(t, s.IsAdmissionEnabled())
}

func TestIsAdmissionEnabled_ExplicitTrue(t *testing.T) {
	tr := true
	s := &types.CatalogSecurity{
		Webhooks: &types.WebhooksConfig{
			Admission: &types.AdmissionWebhookToggle{Enabled: &tr},
		},
	}
	assert.True(t, s.IsAdmissionEnabled())
}

// ── IsConversionEnabled ───────────────────────────────────────────────────────

func TestIsConversionEnabled_Nil(t *testing.T) {
	var s *types.CatalogSecurity
	assert.False(t, s.IsConversionEnabled())
}

func TestIsConversionEnabled_NilConversion(t *testing.T) {
	s := &types.CatalogSecurity{}
	assert.False(t, s.IsConversionEnabled())
}

func TestIsConversionEnabled_ExplicitTrue(t *testing.T) {
	tr := true
	s := &types.CatalogSecurity{Conversion: &types.ConversionConfig{Enabled: &tr}}
	assert.True(t, s.IsConversionEnabled())
}

// ── DeletionProtectionFailurePolicy ──────────────────────────────────────────

func TestDeletionProtectionFailurePolicy_DefaultFail(t *testing.T) {
	var s *types.CatalogSecurity
	assert.Equal(t, "Fail", s.DeletionProtectionFailurePolicy())
}

func TestDeletionProtectionFailurePolicy_Custom(t *testing.T) {
	s := &types.CatalogSecurity{
		DeletionProtection: &types.DeletionProtectionConfig{FailurePolicy: "Ignore"},
	}
	assert.Equal(t, "Ignore", s.DeletionProtectionFailurePolicy())
}

// ── IsNamespaceProtectionEnabled ──────────────────────────────────────────────

func TestIsNamespaceProtectionEnabled_Nil(t *testing.T) {
	var s *types.CatalogSecurity
	assert.False(t, s.IsNamespaceProtectionEnabled())
}

func TestIsNamespaceProtectionEnabled_DeclaredNoEnabled(t *testing.T) {
	s := &types.CatalogSecurity{NamespaceProtection: &types.NamespaceProtectionConfig{}}
	assert.True(t, s.IsNamespaceProtectionEnabled())
}

func TestIsNamespaceProtectionEnabled_ExplicitFalse(t *testing.T) {
	f := false
	s := &types.CatalogSecurity{
		NamespaceProtection: &types.NamespaceProtectionConfig{Enabled: &f},
	}
	assert.False(t, s.IsNamespaceProtectionEnabled())
}

// ── NamespaceProtectionFailurePolicy ─────────────────────────────────────────

func TestNamespaceProtectionFailurePolicy_DefaultFail(t *testing.T) {
	var s *types.CatalogSecurity
	assert.Equal(t, "Fail", s.NamespaceProtectionFailurePolicy())
}

func TestNamespaceProtectionFailurePolicy_Custom(t *testing.T) {
	s := &types.CatalogSecurity{
		NamespaceProtection: &types.NamespaceProtectionConfig{FailurePolicy: "Ignore"},
	}
	assert.Equal(t, "Ignore", s.NamespaceProtectionFailurePolicy())
}

// ── RuntimeServiceName / GatewayServiceName ──────────────────────

func TestRuntimeServiceName_FallsBackToEnvDefault(t *testing.T) {
	var s *types.CatalogSecurity
	assert.Equal(t, "inrun-runtime", s.RuntimeServiceName("inrun-runtime"))
}

func TestRuntimeServiceName_Custom(t *testing.T) {
	s := &types.CatalogSecurity{ServiceName: &types.ServiceName{Runtime: "my-runtime"}}
	assert.Equal(t, "my-runtime", s.RuntimeServiceName("default"))
}

func TestGatewayServiceName_FallsBackToEnvDefault(t *testing.T) {
	var s *types.CatalogSecurity
	assert.Equal(t, "inrun-gateway", s.GatewayServiceName("inrun-gateway"))
}

func TestGatewayServiceName_Custom(t *testing.T) {
	s := &types.CatalogSecurity{ServiceName: &types.ServiceName{Gateway: "my-gateway"}}
	assert.Equal(t, "my-gateway", s.GatewayServiceName("default"))
}
