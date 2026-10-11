package external

import (
	"context"
	"encoding/json"

	"github.com/intentd/inrun/pkg/types"
)

// httpProtocolClient wraps executeHTTPCall for the ProtocolClient interface.
// All errors (4xx, 5xx, network) are surfaced via entry["error"]; Fetch never
// returns a Go error. The runner reads entry["error"] and enforces continueOnError.
type httpProtocolClient struct{}

func (c *httpProtocolClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, _, resolvedBody, credential string) (map[string]interface{}, error) {
	authHeader := "Authorization"
	if spec.Auth != nil && spec.Auth.Header != "" {
		authHeader = spec.Auth.Header
	}

	result := executeHTTPCall(ctx, spec, resolvedURL, resolvedBody, credential, authHeader)

	entry := map[string]interface{}{}
	if result.Body != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result.Body), &parsed); err == nil {
			for k, v := range parsed {
				entry[k] = v
			}
		}
	}
	// HTTP meta keys set after JSON so they're never overwritten by a body key
	// with the same name (e.g. {"status": "ok"}).
	entry["status"] = result.Status
	entry["body"] = result.Body
	entry["error"] = result.Error
	entry["called"] = "true"
	return entry, nil
}
