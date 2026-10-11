package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithPublish(pub *types.PublishConfig) *executor {
	return newExec(&catalog.Catalog{Publish: pub})
}

// ── validatePublish ──────────────────────────────────────────────────────────

func TestValidatePublish_Nil(t *testing.T) {
	k := catalogWithPublish(nil)
	assert.NoError(t, k.validatePublish())
}

func TestValidatePublish_EmptyConfig(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{})
	assert.NoError(t, k.validatePublish())
}

// ── publish.tests.intent ─────────────────────────────────────────────────────

func TestValidatePublish_Tests_IntentNil(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{},
	})
	assert.NoError(t, k.validatePublish())
}

func TestValidatePublish_Tests_IntentFalse(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(false)},
	})
	assert.NoError(t, k.validatePublish())
}

func TestValidatePublish_Tests_IntentTrue_NoGateway(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	err := k.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent")
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_GatewayNoAPIBlock(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	k.k.Gateway = &types.GatewayConfig{}
	err := k.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_GatewayAPIDisabled(t *testing.T) {
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	k.k.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: false},
	}
	err := k.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_NoIntentFiles(t *testing.T) {
	dir := t.TempDir()
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	k.k.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	k.k.SetCatalogDirForTest(dir)
	err := k.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent.yaml")
	assert.Contains(t, err.Error(), "intent.json")
}

func TestValidatePublish_Tests_IntentTrue_WithIntentYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "intent.yaml"), []byte("{}"), 0o644))
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	k.k.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	k.k.SetCatalogDirForTest(dir)
	assert.NoError(t, k.validatePublish())
}

func TestValidatePublish_Tests_IntentTrue_WithIntentJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "intent.json"), []byte("{}"), 0o644))
	k := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	k.k.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	k.k.SetCatalogDirForTest(dir)
	assert.NoError(t, k.validatePublish())
}
