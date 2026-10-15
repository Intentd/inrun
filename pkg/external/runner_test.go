package external_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// stubTransport returns a fixed response for every request.
type stubTransport struct {
	status int
	body   string
	err    error
}

func (s stubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func withTransport(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()
	prev := external.HTTPTransport
	external.HTTPTransport = transport
	return func() { external.HTTPTransport = prev }
}

func resolver(data map[string]interface{}) *template.Resolver {
	return template.NewResolverFromMap(data)
}

func TestRun_EmptyCalls(t *testing.T) {
	r := resolver(map[string]interface{}{})
	got, err := external.Run(context.Background(), "test", r, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != r {
		t.Fatal("expected same resolver when calls is nil")
	}
}

func TestRun_SuccessfulCall(t *testing.T) {
	defer withTransport(t, stubTransport{status: 200, body: `{"enabled":true}`})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "flags", URL: "http://flags.internal/v1"},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := got.Data()
	ext, ok := data["external"].(map[string]interface{})
	if !ok {
		t.Fatal("expected .external in resolver data")
	}
	flags, ok := ext["flags"].(map[string]interface{})
	if !ok {
		t.Fatal("expected .external.flags")
	}
	if flags["status"] != "200" {
		t.Errorf("status: got %v, want 200", flags["status"])
	}
	if flags["called"] != "true" {
		t.Errorf("called: got %v, want true", flags["called"])
	}
}

func TestRun_JSONBodyDecoded(t *testing.T) {
	defer withTransport(t, stubTransport{status: 200, body: `{"enabled":true,"limit":5}`})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "cfg", URL: "http://config.internal/v1"},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := got.Data()
	ext := data["external"].(map[string]interface{})
	cfg := ext["cfg"].(map[string]interface{})

	// the decoded body is under json; nothing is merged into the entry
	body, ok := cfg["json"].(map[string]interface{})
	if !ok {
		t.Fatalf("json: got %T, want the decoded object", cfg["json"])
	}
	if body["enabled"] != true {
		t.Errorf("json.enabled: got %v, want true", body["enabled"])
	}
	if fmt.Sprintf("%v", body["limit"]) != "5" {
		t.Errorf("json.limit: got %v, want 5", body["limit"])
	}
	if _, merged := cfg["enabled"]; merged {
		t.Error("body keys must not be merged into the entry")
	}
}

func TestRun_JSONArrayBodyDecoded(t *testing.T) {
	defer withTransport(t, stubTransport{status: 200, body: `[{"id":"a"},{"id":"b"}]`})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{{Name: "list", URL: "http://api.internal/items"}}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry := got.Data()["external"].(map[string]interface{})["list"].(map[string]interface{})
	items, ok := entry["json"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("json: got %v, want a two-item array", entry["json"])
	}
}

func TestRun_ContinueOnError_True(t *testing.T) {
	defer withTransport(t, stubTransport{status: 503, body: "unavailable"})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "svc", URL: "http://svc.internal/health", ContinueOnError: true},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("expected no error with continueOnError:true, got: %v", err)
	}

	ext := got.Data()["external"].(map[string]interface{})
	svc := ext["svc"].(map[string]interface{})
	if svc["error"] == "" {
		t.Error("expected error to be set on failed call")
	}
}

func TestRun_ContinueOnError_False(t *testing.T) {
	defer withTransport(t, stubTransport{status: 503, body: "unavailable"})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "svc", URL: "http://svc.internal/health", ContinueOnError: false},
	}

	_, err := external.Run(context.Background(), "test", r, calls, nil)
	if err == nil {
		t.Fatal("expected error with continueOnError:false on a failed call")
	}
}

func TestRun_WhenConditionSkipsCall(t *testing.T) {
	called := false
	defer withTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return nil, fmt.Errorf("should not be called")
	}))()

	r := resolver(map[string]interface{}{
		"spec": map[string]interface{}{"env": "staging"},
	})
	calls := []types.ExternalCallSpec{
		{
			Name: "prod-only",
			URL:  "http://prod.internal/v1",
			Conditions: []types.Condition{
				{Field: "spec.env", Equals: "production"},
			},
		},
	}

	_, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("HTTP call should have been skipped but transport was hit")
	}
}

func TestRun_LaterCallSeesEarlierResult(t *testing.T) {
	// First call returns a URL; second call uses it in its own URL template.
	callCount := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var body string
		switch callCount {
		case 1:
			body = `{"endpoint":"http://actual.internal/data"}`
		case 2:
			// second call — just confirm it was made to the resolved URL
			body = `{"value":"ok"}`
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})
	defer withTransport(t, transport)()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "discovery", URL: "http://discovery.internal/v1"},
		{Name: "data", URL: `{{ index .external "discovery" "endpoint" }}`},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}

	ext := got.Data()["external"].(map[string]interface{})
	if ext["data"] == nil {
		t.Error("expected .external.data to be present")
	}
}

func TestRun_ExpectedStatusMismatch(t *testing.T) {
	defer withTransport(t, stubTransport{status: 201, body: ""})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "api", URL: "http://api.internal/v1", ExpectedStatus: []int{200}, ContinueOnError: false},
	}

	_, err := external.Run(context.Background(), "test", r, calls, nil)
	if err == nil {
		t.Fatal("expected error when response status doesn't match expectedStatus")
	}
}

func TestRun_JSONStatusKeyStaysReadable(t *testing.T) {
	// A body field named like a result key ("status") is read from json and
	// never overwrites the HTTP status code.
	defer withTransport(t, stubTransport{status: 200, body: `{"status":"ok"}`})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "health", URL: "http://svc.internal/health"},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ext := got.Data()["external"].(map[string]interface{})
	health := ext["health"].(map[string]interface{})
	if health["status"] != "200" {
		t.Errorf("status: got %v, want 200 — JSON body key must not overwrite HTTP status", health["status"])
	}
	if body := health["json"].(map[string]interface{}); body["status"] != "ok" {
		t.Errorf("json.status: got %v, want ok", body["status"])
	}
}

func TestRun_NonJSONBodyNotParsed(t *testing.T) {
	defer withTransport(t, stubTransport{status: 200, body: "plain text response"})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "txt", URL: "http://txt.internal/v1"},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ext := got.Data()["external"].(map[string]interface{})
	entry := ext["txt"].(map[string]interface{})
	if entry["body"] != "plain text response" {
		t.Errorf("body: got %v, want 'plain text response'", entry["body"])
	}
	if _, ok := entry["json"]; ok {
		t.Error("a non-JSON body has no json key")
	}
}

// roundTripFunc allows using a function as an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRun_ExpectedStatusList(t *testing.T) {
	defer withTransport(t, stubTransport{status: 404, body: ""})()

	r := resolver(map[string]interface{}{})
	calls := []types.ExternalCallSpec{
		{Name: "read", URL: "http://api.internal/v1/item", ExpectedStatus: []int{200, 404}},
	}

	got, err := external.Run(context.Background(), "test", r, calls, nil)
	if err != nil {
		t.Fatalf("404 is listed in expectedStatus, so the call succeeds: %v", err)
	}
	entry := got.Data()["external"].(map[string]interface{})["read"].(map[string]interface{})
	if entry["status"] != "404" || entry["error"] != "" {
		t.Errorf("got status %v error %q, want 404 and no error", entry["status"], entry["error"])
	}
}

func TestValidateCall_ExpectedStatusRange(t *testing.T) {
	call := types.ExternalCallSpec{Name: "read", URL: "http://api.internal", ExpectedStatus: []int{200, 99}}
	if err := external.ValidateCall("test", call); err == nil {
		t.Fatal("99 is not an HTTP status code")
	}
	call.ExpectedStatus = []int{200, 404}
	if err := external.ValidateCall("test", call); err != nil {
		t.Fatalf("200 and 404 are valid: %v", err)
	}
}

func TestCall_OneCall(t *testing.T) {
	defer withTransport(t, stubTransport{status: 200, body: `{"id":"b-1"}`})()
	r := resolver(map[string]interface{}{"spec": map[string]interface{}{"ready": "no"}})

	entry, err := external.Call(context.Background(), "test", "remote", r,
		types.ExternalCallSpec{Name: "ensure", URL: "http://api.internal/buckets/b-1", Method: "PUT"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry["status"] != "200" || entry["json"].(map[string]interface{})["id"] != "b-1" {
		t.Errorf("got %v", entry)
	}

	skipped, err := external.Call(context.Background(), "test", "remote", r, types.ExternalCallSpec{
		Name: "gated", URL: "http://api.internal/x",
		Conditions: []types.Condition{{Field: "spec.ready", Equals: "yes"}},
	}, nil)
	if err != nil || skipped["called"] != "false" {
		t.Errorf("a call whose conditions fail is skipped: %v, %v", skipped, err)
	}
}

func TestCall_FailureReturnsEntryAndError(t *testing.T) {
	defer withTransport(t, stubTransport{status: 503, body: `{"reason":"busy"}`})()
	r := resolver(map[string]interface{}{})

	entry, err := external.Call(context.Background(), "test", "remote", r,
		types.ExternalCallSpec{Name: "ensure", URL: "http://api.internal/x"}, nil)
	if err == nil {
		t.Fatal("a 503 without continueOnError is an error")
	}
	if entry == nil || entry["status"] != "503" {
		t.Errorf("the failed call's entry is still returned: %v", entry)
	}
}
