package external

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// RequestHeaders returns spec's headers for one request: each value rendered
// with render, plus the idempotency key on POST and PATCH. identity names
// the CR (ObjectIdentity); method, url and body are the request as sent.
// A header the user declares under the key's name wins over the generated
// key.
func RequestHeaders(render func(string) (string, error), spec types.CallSpec, identity, method, url, body string) (map[string]string, error) {
	out := make(map[string]string, len(spec.Headers)+1)
	for name, value := range spec.Headers {
		rendered, err := render(value)
		if err != nil {
			return nil, fmt.Errorf("headers.%s: %w", name, err)
		}
		out[name] = rendered
	}

	cfg := spec.IdempotencyKey
	if cfg.IsDisabled() || !needsIdempotencyKey(method) {
		return out, nil
	}
	header := cfg.EffectiveHeader()
	for name := range out {
		if strings.EqualFold(name, header) {
			return out, nil
		}
	}
	key := IdempotencyKey(identity, method, url, body)
	if cfg != nil && cfg.Value != "" {
		rendered, err := render(cfg.Value)
		if err != nil {
			return nil, fmt.Errorf("idempotencyKey.value: %w", err)
		}
		key = rendered
	}
	out[header] = key
	return out, nil
}

// IdempotencyKey derives the key for a request: the first 32 hex characters
// of a SHA-256 over the CR's identity, the method, the URL and the body. A
// retry of the same request repeats the key; a changed spec, which renders a
// different body or URL, gets a new one; two CRs never share one.
func IdempotencyKey(identity, method, url, body string) string {
	sum := sha256.Sum256([]byte(identity + "\n" + strings.ToUpper(method) + "\n" + url + "\n" + body))
	return hex.EncodeToString(sum[:])[:32]
}

// ObjectIdentity names the CR in data (a template context or an object
// map): its UID, or namespace/name before the API server has assigned one,
// as on a create at admission time.
func ObjectIdentity(data map[string]interface{}) string {
	meta, _ := data["metadata"].(map[string]interface{})
	if uid, _ := meta["uid"].(string); uid != "" {
		return uid
	}
	ns, _ := meta["namespace"].(string)
	name, _ := meta["name"].(string)
	return ns + "/" + name
}

// needsIdempotencyKey reports whether method is one HTTP does not already
// make safe to repeat.
func needsIdempotencyKey(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPatch:
		return true
	}
	return false
}
