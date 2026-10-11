package publish

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithPublish(pub *types.PublishConfig) *checker {
	return newExec(&catalog.Catalog{Publish: pub})
}

// ── validatePublish ──────────────────────────────────────────────────────────

func TestValidatePublish_Nil(t *testing.T) {
	e := catalogWithPublish(nil)
	assert.NoError(t, e.validatePublish())
}

func TestValidatePublish_EmptyConfig(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{})
	assert.NoError(t, e.validatePublish())
}

// ── publish.tests.intent ─────────────────────────────────────────────────────

func TestValidatePublish_Tests_IntentNil(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{},
	})
	assert.NoError(t, e.validatePublish())
}

func TestValidatePublish_Tests_IntentFalse(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(false)},
	})
	assert.NoError(t, e.validatePublish())
}

func TestValidatePublish_Tests_IntentTrue_NoGateway(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	err := e.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent")
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_GatewayNoAPIBlock(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	e.Cat.Gateway = &types.GatewayConfig{}
	err := e.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_GatewayAPIDisabled(t *testing.T) {
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	e.Cat.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: false},
	}
	err := e.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway.api.enabled")
}

func TestValidatePublish_Tests_IntentTrue_NoIntentFiles(t *testing.T) {
	dir := t.TempDir()
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	e.Cat.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	e.Cat.SetCatalogDirForTest(dir)
	err := e.validatePublish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent.yaml")
	assert.Contains(t, err.Error(), "intent.json")
}

func TestValidatePublish_Tests_IntentTrue_WithIntentYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "intent.yaml"), []byte("{}"), 0o644))
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	e.Cat.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	e.Cat.SetCatalogDirForTest(dir)
	assert.NoError(t, e.validatePublish())
}

func TestValidatePublish_Tests_IntentTrue_WithIntentJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "intent.json"), []byte("{}"), 0o644))
	e := catalogWithPublish(&types.PublishConfig{
		Tests: &types.PublishTestsConfig{Intent: boolPtr(true)},
	})
	e.Cat.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{Enabled: true},
	}
	e.Cat.SetCatalogDirForTest(dir)
	assert.NoError(t, e.validatePublish())
}
