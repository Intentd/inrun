package types

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func strictYAML(t *testing.T, src string, out interface{}) error {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader([]byte(src)))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func TestIdempotencyKeyConfig_YAML(t *testing.T) {
	var off struct {
		K *IdempotencyKeyConfig `yaml:"idempotencyKey"`
	}
	require.NoError(t, strictYAML(t, "idempotencyKey: false\n", &off))
	assert.True(t, off.K.IsDisabled())

	var custom struct {
		K *IdempotencyKeyConfig `yaml:"idempotencyKey"`
	}
	require.NoError(t, strictYAML(t, "idempotencyKey: {header: PayPal-Request-Id, value: \"{{ .metadata.name }}\"}\n", &custom))
	assert.False(t, custom.K.IsDisabled())
	assert.Equal(t, "PayPal-Request-Id", custom.K.EffectiveHeader())
	assert.Equal(t, "{{ .metadata.name }}", custom.K.Value)

	var on struct {
		K *IdempotencyKeyConfig `yaml:"idempotencyKey"`
	}
	assert.Error(t, strictYAML(t, "idempotencyKey: true\n", &on), "true is not a setting: absent already means on")

	var nilCfg *IdempotencyKeyConfig
	assert.Equal(t, DefaultIdempotencyHeader, nilCfg.EffectiveHeader())
	assert.False(t, nilCfg.IsDisabled())
}

func TestIdempotencyKeyConfig_JSON(t *testing.T) {
	var c IdempotencyKeyConfig
	require.NoError(t, json.Unmarshal([]byte("false"), &c))
	assert.True(t, c.Disabled)
	b, err := json.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, "false", string(b))

	require.NoError(t, json.Unmarshal([]byte(`{"header":"X-Key"}`), &c))
	assert.Equal(t, IdempotencyKeyConfig{Header: "X-Key"}, c)
}

func TestExternalCallSpec_InlineCallFields(t *testing.T) {
	var call ExternalCallSpec
	require.NoError(t, strictYAML(t, `
name: create
url: "https://api.example/buckets"
method: POST
headers: {X-Tenant: "{{ .metadata.namespace }}"}
body: '{"name": "logs"}'
expectedStatus: [201]
when: [{field: spec.enabled, equals: "true"}]
timeout: 5s
idempotencyKey: false
continueOnError: true
`, &call))
	assert.Equal(t, "create", call.Name)
	assert.Equal(t, "https://api.example/buckets", call.URL)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "{{ .metadata.namespace }}", call.Headers["X-Tenant"])
	assert.Equal(t, []int{201}, call.ExpectedStatus)
	assert.Len(t, call.Conditions, 1)
	assert.True(t, call.IdempotencyKey.IsDisabled())
	assert.True(t, call.ContinueOnError)
}

func TestRemoteRequestConfig_FinalizeURL(t *testing.T) {
	var rq RemoteRequestConfig
	require.NoError(t, strictYAML(t, `
method: POST
headers: {X-Tenant: acme}
finalize:
  url: "{{ api }}/buckets/{{ .status.bucketId }}"
  method: DELETE
  expectedStatus: [202, 404]
`, &rq))
	assert.Equal(t, "POST", rq.Method)
	assert.Equal(t, "acme", rq.Headers["X-Tenant"])
	require.NotNil(t, rq.Finalize)
	assert.Equal(t, "{{ api }}/buckets/{{ .status.bucketId }}", rq.Finalize.URL)

	var old RemoteRequestConfig
	assert.Error(t, strictYAML(t, "finalize:\n  endpoint: x\n", &old), "finalize.endpoint was renamed to finalize.url")
}
