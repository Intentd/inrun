// Package contract defines the transport-agnostic types shared across all
// remote reconciler implementations (http, grpc, …).
package contract

import (
	"encoding/json"

	"github.com/inrundev/inrun/domain"
)

// Result is the response a remote reconciler returns after each reconcile
// cycle. One JSON object holds both the wire envelope below and a
// domain.Result in its JSON form (requeue, requeueAfter, cleanup, status,
// resources).
type Result struct {
	// Result is "ok" (or empty) or "error".
	Result string `json:"result"`
	// Error is a human-readable error message, used with result "error".
	// Triggers backoff retry.
	Error string `json:"error,omitempty"`
	// Directives is the rest of the response: what the reconciler asks the
	// runtime to do next.
	Directives domain.Result `json:"-"`
}

// Decode reads a remote reconciler's JSON response.
func Decode(data []byte) (Result, error) {
	var res Result
	if err := json.Unmarshal(data, &res); err != nil {
		return Result{}, err
	}
	// The same object also holds the directives. They are decoded separately:
	// domain.Result has its own UnmarshalJSON, which embedding would promote
	// over the whole object and lose result/error. On an error response the
	// directives are ignored, so a malformed one cannot hide the error message.
	if res.Result == "error" {
		return res, nil
	}
	if err := json.Unmarshal(data, &res.Directives); err != nil {
		return Result{}, err
	}
	return res, nil
}
