package funcs

import (
	"encoding/json"
	"strings"
)

// jsonValueSentinel prefixes a resolved string whose payload is JSON-encoded
// data rather than a plain string. Inrun's resolver path narrows every value
// to a string, so a function that produces a structured value (a map, a list)
// must encode it and mark it with this prefix; resolveValue in
// pkg/gateway/webhook/conversion_logic.go detects the prefix and decodes the payload back
// to its structured form.
//
// Null bytes are used because they cannot appear in normal YAML field values
const jsonValueSentinel = "\x00JSON\x00"

// HasJSONValueSentinel reports whether s begins with the JSON value sentinel
func HasJSONValueSentinel(s string) bool {
	return strings.HasPrefix(s, jsonValueSentinel)
}

// EncodeJSONValueSentinel returns v encoded as JSON and prefixed with the
// sentinel, so a function producing a structured value can return it through
// the string-only resolver path. Use with HasJSONValueSentinel and
// DecodeJSONValueSentinel on the consumer side.
func EncodeJSONValueSentinel(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return jsonValueSentinel + string(b), nil
}

// DecodeJSONValueSentinel decodes the JSON payload in s into out. Callers must
// first check HasJSONValueSentinel(s); this does not test for the sentinel and
// returns an error only for a malformed payload.
func DecodeJSONValueSentinel(s string, out interface{}) error {
	payload := strings.TrimPrefix(s, jsonValueSentinel)
	return json.Unmarshal([]byte(payload), out)
}
