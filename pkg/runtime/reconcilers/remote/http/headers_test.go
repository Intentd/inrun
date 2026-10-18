package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerAPI records the headers of each request and answers POST with 201
// and an id, DELETE with 202.
func headerAPI(t *testing.T) (*httptest.Server, *[]http.Header) {
	t.Helper()
	var seen []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Clone())
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"b-1"}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func requestWithUID(finalizing bool) domain.Request {
	resolver := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "logs", "namespace": "team-a", "uid": "uid-1"},
	})
	return domain.Request{Key: "team-a/logs", Prepared: &domain.PreparedRequest{Context: resolver, Finalizing: finalizing}}
}

func TestMapped_PostSendsHeadersAndIdempotencyKey(t *testing.T) {
	srv, seen := headerAPI(t)
	r := &Reconciler{timeout: 5 * time.Second, decl: &types.RemoteReconcilerDeclaration{
		Endpoint: srv.URL + "/buckets",
		Request: &types.RemoteRequestConfig{
			CallSpec: types.CallSpec{
				Method:         http.MethodPost,
				Headers:        map[string]string{"X-Tenant": "{{ .metadata.namespace }}"},
				Body:           `{"name": "{{ .metadata.name }}"}`,
				ExpectedStatus: []int{201},
			},
			Finalize: &types.CallSpec{URL: srv.URL + "/buckets/b-1", Method: http.MethodDelete, ExpectedStatus: []int{202}},
		},
		Response: &types.RemoteResponseConfig{},
	}}

	_, err := r.reconcileMapped(context.Background(), requestWithUID(false))
	require.NoError(t, err)
	_, err = r.reconcileMapped(context.Background(), requestWithUID(false))
	require.NoError(t, err)
	_, err = r.reconcileMapped(context.Background(), requestWithUID(true))
	require.NoError(t, err)

	require.Len(t, *seen, 3)
	create, retry, del := (*seen)[0], (*seen)[1], (*seen)[2]
	assert.Equal(t, "team-a", create.Get("X-Tenant"), "header values are templates")
	assert.Len(t, create.Get(types.DefaultIdempotencyHeader), 32)
	assert.Equal(t, create.Get(types.DefaultIdempotencyHeader), retry.Get(types.DefaultIdempotencyHeader),
		"a retried create repeats its key, so the service returns the first bucket")
	assert.Equal(t, "team-a", del.Get("X-Tenant"), "finalize inherits the request's headers")
	assert.Empty(t, del.Get(types.DefaultIdempotencyHeader), "DELETE is already safe to repeat")
}

func TestContract_SendsHeadersAndIdempotencyKey(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	r := &Reconciler{client: srv.Client(), decl: &types.RemoteReconcilerDeclaration{
		Endpoint: srv.URL,
		Request:  &types.RemoteRequestConfig{CallSpec: types.CallSpec{Headers: map[string]string{"X-Tenant": "{{ .metadata.namespace }}"}}},
	}}

	_, err := r.callRemote(context.Background(), requestWithUID(false))
	require.NoError(t, err)
	assert.Equal(t, "team-a", got.Get("X-Tenant"))
	assert.Len(t, got.Get(types.DefaultIdempotencyHeader), 32, "the contract call is a POST")
}
