package funcs

import (
	"testing"

	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONValueSentinel_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
	}{
		{"map", map[string]interface{}{"minute": "*/5", "hour": "0"}},
		{"slice", []interface{}{"a", "b", "c"}},
		{"string", "plain string"},
		{"int", 42},
		{"bool", true},
		{"nil", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeJSONValueSentinel(tc.in)
			require.NoError(t, err)
			require.True(t, HasJSONValueSentinel(encoded))

			var got interface{}
			require.NoError(t, DecodeJSONValueSentinel(encoded, &got))

			// got is JSON-native by construction (it came out of
			// json.Unmarshal); normalize in so both sides share the same
			// numeric types — e.g. int(42) → float64(42).
			assert.Equal(t, structured.JSONNormalize(tc.in), got)
		})
	}
}

func TestHasJSONValueSentinel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"plain string", "plain string", false},
		{"empty", "", false},
		{"truncated prefix", "\x00JSON", false},
		{"sentinel only", jsonValueSentinel, true},
		{"sentinel with payload", jsonValueSentinel + `{"a":1}`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HasJSONValueSentinel(tc.in))
		})
	}
}

func TestDecodeJSONValueSentinel_InvalidPayload(t *testing.T) {
	var out interface{}
	err := DecodeJSONValueSentinel(jsonValueSentinel+"{not json", &out)
	require.Error(t, err)
}

func TestDecodeJSONValueSentinel_NoSentinel(t *testing.T) {
	// Decode assumes the caller has already checked HasJSONValueSentinel.
	// On a plain string, TrimPrefix is a no-op and json.Unmarshal fails —
	// which is the correct failure mode: misuse surfaces as an error, not
	// silent success.
	var out interface{}
	err := DecodeJSONValueSentinel("plain string", &out)
	require.Error(t, err)
}

func TestEncodeJSONValueSentinel_UnmarshalableInput(t *testing.T) {
	// A channel cannot be JSON-marshalled; Encode must return an error
	// rather than a sentinel with no valid payload.
	_, err := EncodeJSONValueSentinel(make(chan int))
	require.Error(t, err)
}

func TestJSONValueSentinel_Protocol(t *testing.T) {
	// The round-trip property the resolver path depends on: any value that
	// survives Encode also passes Has and Decode.
	payload := map[string]interface{}{"minute": "*/5", "hour": "0"}
	encoded, err := EncodeJSONValueSentinel(payload)
	require.NoError(t, err)

	require.True(t, HasJSONValueSentinel(encoded))

	var decoded map[string]interface{}
	require.NoError(t, DecodeJSONValueSentinel(encoded, &decoded))
	assert.Equal(t, map[string]interface{}{"minute": "*/5", "hour": "0"}, decoded)
}
