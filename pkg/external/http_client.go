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
	// A JSON body is also available decoded, as .external.<name>.json: an
	// object, array or scalar, read like any other value. A body that is not
	// JSON has no json key; body always holds the raw text.
	if result.Body != "" {
		var parsed interface{}
		if err := json.Unmarshal([]byte(result.Body), &parsed); err == nil {
			entry["json"] = parsed
		}
	}
	entry["status"] = result.Status
	entry["body"] = result.Body
	entry["error"] = result.Error
	entry["called"] = "true"
	return entry, nil
}
