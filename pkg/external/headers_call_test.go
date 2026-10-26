package external_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/types"
)

// recordingTransport keeps each request's headers and answers 201.
type recordingTransport struct{ seen *[]http.Header }

func (t recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*t.seen = append(*t.seen, r.Header.Clone())
	return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
}

func TestRun_PostSendsRenderedHeadersAndKey(t *testing.T) {
	var seen []http.Header
	defer withTransport(t, recordingTransport{seen: &seen})()

	r := resolver(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "logs", "namespace": "team-a", "uid": "uid-1"},
	})
	calls := []types.ExternalCallSpec{
		{Name: "create", CallSpec: types.CallSpec{
			URL: "http://buckets.internal/buckets", Method: "POST",
			Headers: map[string]string{"X-Tenant": "{{ .metadata.namespace }}"},
			Body:    `{"name": "{{ .metadata.name }}"}`,
		}},
		{Name: "read", CallSpec: types.CallSpec{URL: "http://buckets.internal/buckets/logs"}},
	}
	if _, err := external.Run(context.Background(), "test", r, calls, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("got %d requests, want 2", len(seen))
	}
	if got := seen[0].Get("X-Tenant"); got != "team-a" {
		t.Errorf("X-Tenant: got %q, want the rendered team-a", got)
	}
	want := external.IdempotencyKey("uid-1", "POST", "http://buckets.internal/buckets", `{"name": "logs"}`)
	if got := seen[0].Get(types.DefaultIdempotencyHeader); got != want {
		t.Errorf("Idempotency-Key: got %q, want %q", got, want)
	}
	if got := seen[1].Get(types.DefaultIdempotencyHeader); got != "" {
		t.Errorf("GET sent a key: %q", got)
	}
}
