package external

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// HTTPTransport is the http.RoundTripper used for all external: HTTP calls.
// nil uses http.DefaultTransport (production). Set to a stub in tests or
// simulation to prevent real network calls.
var HTTPTransport http.RoundTripper

const (
	maxBodyBytes           = 4096
	defaultExternalTimeout = 10 * time.Second
)

// ExpandEnv expands $VAR and ${VAR} environment variable references.
func ExpandEnv(s string) string {
	return os.ExpandEnv(s)
}

// executeHTTPCall makes one HTTP request and returns the result.
// Never panics. Returns an error result on failure.
func executeHTTPCall(
	ctx context.Context,
	spec types.ExternalCallSpec,
	url, body, credential, authHeader string,
) types.ExternalCallResult {
	timeout := defaultExternalTimeout
	if spec.Timeout != "" {
		if d, err := utils.ParseTimeDuration(spec.Timeout); err == nil {
			timeout = d
		}
	}

	method := spec.Method
	if method == "" {
		method = http.MethodGet
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(callCtx, method, url, bodyReader)
	if err != nil {
		return types.ExternalCallResult{
			Error:  fmt.Sprintf("building request: %v", err),
			Called: "true",
		}
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		req.Header.Set(authHeader, "Bearer "+credential)
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()

	var client *http.Client
	if HTTPTransport != nil {
		client = &http.Client{Timeout: timeout, Transport: HTTPTransport}
	} else {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return types.ExternalCallResult{
			Error:  fmt.Sprintf("executing request: %v", err),
			Called: "true",
		}
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return types.ExternalCallResult{
			Status: fmt.Sprintf("%d", resp.StatusCode),
			Error:  fmt.Sprintf("reading response body: %v", err),
			Called: "true",
		}
	}

	durationSeconds := time.Since(start).Seconds()
	statusCode := fmt.Sprintf("%d", resp.StatusCode)
	callErr := ""

	if spec.ExpectedStatus > 0 && resp.StatusCode != spec.ExpectedStatus {
		callErr = fmt.Sprintf("expected status %d, got %d", spec.ExpectedStatus, resp.StatusCode)
	} else if resp.StatusCode >= 400 && spec.ExpectedStatus == 0 {
		callErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return types.ExternalCallResult{
		Status:          statusCode,
		StatusCode:      resp.StatusCode,
		Body:            string(rawBody),
		Error:           callErr,
		Called:          "true",
		DurationSeconds: durationSeconds,
	}
}
