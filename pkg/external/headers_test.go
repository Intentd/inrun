package external

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

// upper renders a template by upper-casing it, so tests can see a value was
// rendered without a resolver.
func upper(v string) (string, error) { return strings.ToUpper(v), nil }

func TestRequestHeaders_RendersValues(t *testing.T) {
	h, err := RequestHeaders(upper, types.CallSpec{Headers: map[string]string{"X-Tenant": "acme"}}, "uid-1", "GET", "u", "")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"X-Tenant": "ACME"}, h, "no key on GET")
}

func TestRequestHeaders_KeyOnlyForPostAndPatch(t *testing.T) {
	for _, m := range []string{"POST", "patch"} {
		h, err := RequestHeaders(upper, types.CallSpec{}, "uid-1", m, "u", "b")
		require.NoError(t, err)
		assert.Len(t, h[types.DefaultIdempotencyHeader], 32, m)
	}
	for _, m := range []string{"GET", "PUT", "DELETE", "HEAD"} {
		h, err := RequestHeaders(upper, types.CallSpec{}, "uid-1", m, "u", "b")
		require.NoError(t, err)
		assert.Empty(t, h, m)
	}
}

func TestIdempotencyKey_RepeatsForTheSameRequestOnly(t *testing.T) {
	base := IdempotencyKey("uid-1", "POST", "https://api/buckets", `{"q":1}`)
	assert.Equal(t, base, IdempotencyKey("uid-1", "post", "https://api/buckets", `{"q":1}`), "a retry repeats the key")
	assert.NotEqual(t, base, IdempotencyKey("uid-1", "POST", "https://api/buckets", `{"q":2}`), "a changed spec gets a new key")
	assert.NotEqual(t, base, IdempotencyKey("uid-2", "POST", "https://api/buckets", `{"q":1}`), "two CRs never share a key")
	assert.NotEqual(t, base, IdempotencyKey("uid-1", "POST", "https://api/other", `{"q":1}`))
}

func TestRequestHeaders_KeyOverrides(t *testing.T) {
	h, err := RequestHeaders(upper, types.CallSpec{IdempotencyKey: &types.IdempotencyKeyConfig{Header: "PayPal-Request-Id", Value: "k-{{x}}"}}, "uid", "POST", "u", "b")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"PayPal-Request-Id": "K-{{X}}"}, h)

	h, err = RequestHeaders(upper, types.CallSpec{IdempotencyKey: &types.IdempotencyKeyConfig{Disabled: true}}, "uid", "POST", "u", "b")
	require.NoError(t, err)
	assert.Empty(t, h)

	h, err = RequestHeaders(upper, types.CallSpec{Headers: map[string]string{"idempotency-key": "mine"}}, "uid", "POST", "u", "b")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"idempotency-key": "MINE"}, h, "the user's own header wins, whatever its case")
}

func TestObjectIdentity(t *testing.T) {
	assert.Equal(t, "uid-1", ObjectIdentity(map[string]interface{}{"metadata": map[string]interface{}{"uid": "uid-1", "name": "logs"}}))
	assert.Equal(t, "default/logs", ObjectIdentity(map[string]interface{}{"metadata": map[string]interface{}{"namespace": "default", "name": "logs"}}),
		"before the API server assigns a UID")
}
