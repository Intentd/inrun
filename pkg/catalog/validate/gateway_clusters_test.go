package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretRef(name, key string) *types.APISecretRef {
	return &types.APISecretRef{Name: name, Key: key}
}

func catalogWithClusters(clusters map[string]types.GatewayClusterConfig) *executor {
	return newExec(&catalog.Catalog{
		Gateway: &types.GatewayConfig{
			Clusters: &types.GatewayClustersConfig{Entries: clusters},
		},
	})
}

// ── validateGatewayClusters — no gateway ─────────────────────────────────────

func TestValidateGatewayClusters_NoGateway(t *testing.T) {
	k := newExec(&catalog.Catalog{})
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_NoClusters(t *testing.T) {
	k := catalogWithClusters(nil)
	assert.NoError(t, k.ValidateGatewayClusters())
}

// ── kubeconfig (secretRef) form ───────────────────────────────────────────────

func TestValidateGatewayClusters_Kubeconfig_Valid(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint:  "https://prod.internal:6443",
			SecretRef: secretRef("prod-creds", "kubeconfig"),
		},
	})
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_Kubeconfig_MissingEndpoint(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {SecretRef: secretRef("prod-creds", "kubeconfig")},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestValidateGatewayClusters_Kubeconfig_MissingSecretName(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint:  "https://prod.internal:6443",
			SecretRef: secretRef("", "kubeconfig"),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secretRef")
	assert.Contains(t, err.Error(), "name")
}

func TestValidateGatewayClusters_Kubeconfig_MissingSecretKey(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint:  "https://prod.internal:6443",
			SecretRef: secretRef("prod-creds", ""),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secretRef")
	assert.Contains(t, err.Error(), "key")
}

func TestValidateGatewayClusters_Kubeconfig_InsecureRejected(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"dev": {
			Endpoint:  "https://dev.internal:6443",
			SecretRef: secretRef("dev-creds", "kubeconfig"),
			Insecure:  true,
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insecure")
	assert.Contains(t, err.Error(), "tokenRef")
}

// ── bearer token + CA (tokenRef/caRef) form ──────────────────────────────────

func TestValidateGatewayClusters_Token_Valid(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint: "https://prod.internal:6443",
			TokenRef: secretRef("prod-sa-token", "token"),
			CARef:    secretRef("prod-ca", "ca.crt"),
		},
	})
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_Token_InsecureNoCA(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"dev": {
			Endpoint: "https://dev.internal:6443",
			TokenRef: secretRef("dev-sa-token", "token"),
			Insecure: true,
		},
	})
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_Token_MissingCA(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint: "https://prod.internal:6443",
			TokenRef: secretRef("prod-sa-token", "token"),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "caRef")
}

func TestValidateGatewayClusters_Token_CAWithoutToken(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint: "https://prod.internal:6443",
			CARef:    secretRef("prod-ca", "ca.crt"),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenRef")
}

func TestValidateGatewayClusters_Token_MissingTokenKey(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint: "https://prod.internal:6443",
			TokenRef: secretRef("prod-sa-token", ""),
			CARef:    secretRef("prod-ca", "ca.crt"),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenRef")
	assert.Contains(t, err.Error(), "key")
}

// ── mixed credentials ─────────────────────────────────────────────────────────

func TestValidateGatewayClusters_MixedCreds_Rejected(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {
			Endpoint:  "https://prod.internal:6443",
			SecretRef: secretRef("prod-kubeconfig", "kubeconfig"),
			TokenRef:  secretRef("prod-sa-token", "token"),
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidateGatewayClusters_NoCreds_Rejected(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {Endpoint: "https://prod.internal:6443"},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential form is required")
}

// ── serve.clusters ref validation ───────────────────────────────────────────

func TestValidateGatewayClusters_ServeClusters_Valid(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod":    {Endpoint: "https://prod.internal:6443", SecretRef: secretRef("prod-creds", "kubeconfig")},
		"staging": {Endpoint: "https://staging.internal:6443", SecretRef: secretRef("staging-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{"prod", "staging"},
				Namespace: "default",
			},
		},
	})
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_ServeClusters_Undefined(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"staging": {Endpoint: "https://staging.internal:6443", SecretRef: secretRef("staging-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{"prod"},
				Namespace: "default",
			},
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"prod"`)
	assert.Contains(t, err.Error(), "not defined in gateway.clusters")
}

func TestValidateGatewayClusters_ServeClusters_Template(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {Endpoint: "https://prod.internal:6443", SecretRef: secretRef("prod-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{`{{ if eq .request.env "prod" }}prod{{ else }}staging{{ end }}`},
				Namespace: "default",
			},
		},
	})
	// template expressions: parse-check only, not validated against registry
	assert.NoError(t, k.ValidateGatewayClusters())
}

func TestValidateGatewayClusters_ServeClusters_InvalidTemplate(t *testing.T) {
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {Endpoint: "https://prod.internal:6443", SecretRef: secretRef("prod-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{`{{ .request.env }`},
				Namespace: "default",
			},
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template")
}

func TestValidateGatewayClusters_TargetClusters_NotInGateway(t *testing.T) {
	// target.clusters name must exist in gateway.clusters.
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod": {Endpoint: "https://prod.internal:6443", SecretRef: secretRef("prod-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{"prod", "staging"},
				Namespace: "default",
				Target: types.ServeTargetValue{
					Entries: map[string]*types.ServeTargetConfig{
						"primary": {Primary: true, Clusters: []string{"staging"}},
					},
				},
			},
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"staging"`)
	assert.Contains(t, err.Error(), "not defined in gateway.clusters")
}

func TestValidateGatewayClusters_TargetClusters_NotInServeClusters(t *testing.T) {
	// target.clusters entries must be a subset of serve.clusters.
	k := catalogWithClusters(map[string]types.GatewayClusterConfig{
		"prod":    {Endpoint: "https://prod.internal:6443", SecretRef: secretRef("prod-creds", "kubeconfig")},
		"staging": {Endpoint: "https://staging.internal:6443", SecretRef: secretRef("staging-creds", "kubeconfig")},
	})
	k.k.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"widget": {
			Serve: &types.ServeConfig{
				Enabled:   true,
				Clusters:  []string{"prod"},
				Namespace: "default",
				Target: types.ServeTargetValue{
					Entries: map[string]*types.ServeTargetConfig{
						"primary": {Primary: true, Clusters: []string{"staging"}},
					},
				},
			},
		},
	})
	err := k.ValidateGatewayClusters()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"staging"`)
	assert.Contains(t, err.Error(), "not declared in serve.clusters")
}
