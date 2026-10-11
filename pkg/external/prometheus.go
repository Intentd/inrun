package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// prometheusClient executes Prometheus instant queries via the HTTP API.
// Endpoint: GET <url>/api/v1/query?query=<PromQL>
// Read-only — never writes to Prometheus.
type prometheusClient struct{}

type promResponse struct {
	Status    string   `json:"status"`
	Data      promData `json:"data"`
	Error     string   `json:"error"`
	ErrorType string   `json:"errorType"`
}

type promData struct {
	ResultType string        `json:"resultType"`
	Result     []interface{} `json:"result"`
}

func (c *prometheusClient) Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, _, credential string) (map[string]interface{}, error) {
	if resolvedQuery == "" {
		return errorResult("prometheus: query: is required"), nil
	}

	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := utils.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	// If the url: already contains the query path (e.g. behind a sub-path proxy),
	// use it as-is. Otherwise append the standard Prometheus instant query endpoint.
	// https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
	base := strings.TrimRight(resolvedURL, "/")
	var endpoint string
	if strings.Contains(base, "/api/v1/query") {
		endpoint = base
	} else {
		endpoint = base + "/api/v1/query"
	}
	params := url.Values{"query": {resolvedQuery}}
	reqURL := endpoint + "?" + params.Encode()

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return errorResult(fmt.Sprintf("prometheus: building request: %v", err)), nil
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	var client *http.Client
	if HTTPTransport != nil {
		client = &http.Client{Timeout: timeout, Transport: HTTPTransport}
	} else {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return errorResult(fmt.Sprintf("prometheus: request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return errorResult(fmt.Sprintf("prometheus: reading response: %v", err)), nil
	}

	var pr promResponse
	if err := json.Unmarshal(raw, &pr); err != nil {
		return errorResult(fmt.Sprintf("prometheus: parsing response: %v", err)), nil
	}

	if pr.Status != "success" {
		return errorResult(fmt.Sprintf("prometheus: %s: %s", pr.ErrorType, pr.Error)), nil
	}

	result, callErr := extractPromResult(pr.Data)

	// Parse raw response into a navigable map.
	var rawMap map[string]interface{}
	json.Unmarshal(raw, &rawMap) //nolint:errcheck

	entry := map[string]interface{}{
		"result": result,
		"raw":    rawMap,
		"error":  callErr,
		"called": "true",
	}
	return entry, nil
}

// extractPromResult pulls the canonical scalar string from the Prometheus response.
// scalar: result[1]
// vector: first series value[1] (use promSum/promMax FuncMap funcs for aggregation)
// matrix: not supported for instant queries — inrun validate rejects these.
func extractPromResult(data promData) (result, callErr string) {
	switch data.ResultType {
	case "scalar":
		if arr, ok := data.Result[1].(string); ok {
			return arr, ""
		}
		return "", "prometheus: unexpected scalar format"

	case "vector":
		if len(data.Result) == 0 {
			return "", ""
		}
		first, ok := data.Result[0].(map[string]interface{})
		if !ok {
			return "", "prometheus: unexpected vector format"
		}
		vals, ok := first["value"].([]interface{})
		if !ok || len(vals) < 2 {
			return "", "prometheus: unexpected vector value format"
		}
		val, ok := vals[1].(string)
		if !ok {
			return "", "prometheus: unexpected vector value type"
		}
		return val, ""

	case "matrix":
		return "", "prometheus: matrix result not supported — use an instant query (remove range selector)"

	default:
		return "", fmt.Sprintf("prometheus: unknown resultType %q", data.ResultType)
	}
}

func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"result": "",
		"raw":    map[string]interface{}{},
		"error":  msg,
		"called": "true",
	}
}
