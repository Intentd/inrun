package http

import (
	"context"
	"io"
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

// bucketAPI stands in for a service that knows nothing about Inrun: PUT
// ensures a bucket and answers its state, DELETE removes it.
func bucketAPI(t *testing.T, state string, deleteStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, r.Method+" "+r.URL.Path+" "+string(body))
		switch r.Method {
		case http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"b-1","status":"` + state + `","endpoint":"https://b-1.example"}`))
		case http.MethodDelete:
			w.WriteHeader(deleteStatus)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func mappedReconciler(endpoint string) *Reconciler {
	return &Reconciler{
		timeout: 5 * time.Second,
		decl: &types.RemoteReconcilerDeclaration{
			Endpoint: endpoint + "/buckets/{{ .metadata.name }}",
			Request: &types.RemoteRequestConfig{
				Method:         http.MethodPut,
				Body:           `{"quotaGB": {{ .args.quotaGB }}}`,
				ExpectedStatus: []int{200},
				Finalize: &types.RemoteRequestCall{
					Method:         http.MethodDelete,
					ExpectedStatus: []int{202, 404},
				},
			},
			Response: &types.RemoteResponseConfig{
				Status: &types.StatusConfig{Fields: []types.StatusFieldSpec{
					{Path: "bucketId", Value: "{{ .response.json.id }}"},
					{Path: "state", Value: "{{ .response.json.status }}"},
				}},
				Requeue: &types.RequeueConfig{
					After: "10s",
					Or: []types.Condition{
						{Field: "response.json.status", Equals: "provisioning"},
						{Field: "response.status", Equals: "202"},
					},
				},
			},
		},
	}
}

func mappedRequest(finalizing bool) domain.Request {
	resolver := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "logs"},
	}).WithArgs(map[string]interface{}{"quotaGB": 50})
	return domain.Request{Key: "default/logs", Prepared: &domain.PreparedRequest{Context: resolver, Finalizing: finalizing}}
}

func TestMapped_EnsureMapsStatusAndRequeue(t *testing.T) {
	srv, seen := bucketAPI(t, "provisioning", 0)
	r := mappedReconciler(srv.URL)

	result, err := r.reconcileMapped(context.Background(), mappedRequest(false))
	require.NoError(t, err)

	require.Len(t, *seen, 1, "one call per reconcile")
	assert.Equal(t, `PUT /buckets/logs {"quotaGB": 50}`, (*seen)[0], "method, templated path and a body built from .args")
	assert.Equal(t, "b-1", result.Status["bucketId"])
	assert.Equal(t, "provisioning", result.Status["state"])
	require.NotNil(t, result.Requeue, "provisioning asks to come back")
	assert.Equal(t, 10*time.Second, result.Requeue.After)
}

func TestMapped_ResponseReachesTheRequest(t *testing.T) {
	srv, _ := bucketAPI(t, "ready", 0)
	req := mappedRequest(false)
	_, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), req)
	require.NoError(t, err)

	// Templates after the call, emit.status and reconcile.requeue read it.
	out, err := req.Prepared.Context.(*template.Resolver).Resolve("{{ .response.json.id }}")
	require.NoError(t, err)
	assert.Equal(t, "b-1", out)
}

func TestMapped_ReadyNeedsNoRequeue(t *testing.T) {
	srv, _ := bucketAPI(t, "ready", 0)
	result, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), mappedRequest(false))
	require.NoError(t, err)
	assert.Nil(t, result.Requeue)
}

func TestMapped_FinalizeCallsTheDeleteAndHoldsWhileAccepted(t *testing.T) {
	srv, seen := bucketAPI(t, "", http.StatusAccepted)
	result, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), mappedRequest(true))
	require.NoError(t, err)
	assert.Equal(t, "DELETE /buckets/logs ", (*seen)[0])
	require.NotNil(t, result.Requeue, "202 means the delete is still running: hold the CR")
}

func TestMapped_FinalizeDoneReleases(t *testing.T) {
	srv, _ := bucketAPI(t, "", http.StatusNotFound)
	result, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), mappedRequest(true))
	require.NoError(t, err, "404 is in finalize's expectedStatus: gone is done")
	assert.Nil(t, result.Requeue)
}

func TestMapped_EnsureNotFoundFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), mappedRequest(false))
	assert.Error(t, err, "404 means done only for finalize; for the ensure call it is a failure")
}

func TestMapped_UnexpectedStatusFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := mappedReconciler(srv.URL).reconcileMapped(context.Background(), mappedRequest(false))
	assert.Error(t, err)
}

func TestMapped_FinalizingWithoutFinalizeReleases(t *testing.T) {
	r := mappedReconciler("http://unused.invalid")
	r.decl.Request.Finalize = nil
	result, err := r.reconcileMapped(context.Background(), mappedRequest(true))
	require.NoError(t, err)
	assert.Nil(t, result.Requeue, "no call to make: the CR is released")
}

func TestBuildBody_DeclaredBodyReplacesInrunsRequest(t *testing.T) {
	r := newReconcilerForTest("http://svc/reconcile")
	r.decl.Request = &types.RemoteRequestConfig{Body: `{"size": {{ .args.quotaGB }}}`}
	body, err := r.buildBody(mappedRequest(false))
	require.NoError(t, err)
	assert.JSONEq(t, `{"size": 50}`, string(body))
}

func TestHooks_CallDrivesTheResult(t *testing.T) {
	srv, seen := bucketAPI(t, "provisioning", http.StatusAccepted)
	hooks := mappedReconciler(srv.URL).Hooks()
	require.NotNil(t, hooks.OnReconcile)
	require.NotNil(t, hooks.OnDelete, "request.finalize is the OnDelete")

	result, err := hooks.OnReconcile(context.Background(), mappedRequest(false), nil)
	require.NoError(t, err)
	assert.Equal(t, "b-1", result.Status["bucketId"])

	result, err = hooks.OnDelete(context.Background(), mappedRequest(true), nil)
	require.NoError(t, err)
	assert.NotNil(t, result.Requeue, "202 holds the CR")
	assert.Len(t, *seen, 2)
}

func TestHooks_NoFinalizeLeavesOnDeleteToTemplates(t *testing.T) {
	r := mappedReconciler("http://unused")
	r.decl.Request.Finalize = nil
	assert.Nil(t, r.Hooks().OnDelete)
}

// observed is the request with an observe result in scope, as
// reconcile.external leaves it.
func observed(finalizing bool, status string, id string) domain.Request {
	req := mappedRequest(finalizing)
	resolver := req.Prepared.Context.(*template.Resolver).WithExternal(map[string]interface{}{
		"current": map[string]interface{}{"status": status, "json": map[string]interface{}{"id": id}},
	})
	req.Prepared.Context = resolver
	return req
}

func TestMapped_RequestSkippedWhenItsConditionFails(t *testing.T) {
	srv, seen := bucketAPI(t, "ready", 0)
	r := mappedReconciler(srv.URL)
	r.decl.Request.When = []types.Condition{{Field: "external.current.status", Equals: "404"}}
	r.decl.Response.Requeue = &types.RequeueConfig{After: "30s"}

	result, err := r.reconcileMapped(context.Background(), observed(false, "200", "b-1"))
	require.NoError(t, err)
	assert.Empty(t, *seen, "present already: no write")
	assert.Nil(t, result.Status, "no answer to map")
	require.NotNil(t, result.Requeue, "the requeue still applies")
	assert.Equal(t, 30*time.Second, result.Requeue.After)

	_, err = r.reconcileMapped(context.Background(), observed(false, "404", ""))
	require.NoError(t, err)
	assert.Len(t, *seen, 1, "absent: the call is made")
}

func TestMapped_FinalizeSkippedReleases(t *testing.T) {
	srv, seen := bucketAPI(t, "", http.StatusAccepted)
	r := mappedReconciler(srv.URL)
	r.decl.Request.Finalize.When = []types.Condition{{Field: "external.current.status", Equals: "200"}}

	result, err := r.reconcileMapped(context.Background(), observed(true, "404", ""))
	require.NoError(t, err)
	assert.Empty(t, *seen, "already gone: no delete")
	assert.Nil(t, result.Requeue, "released")
}

func TestMapped_FinalizeEndpoint(t *testing.T) {
	srv, seen := bucketAPI(t, "", http.StatusAccepted)
	r := mappedReconciler(srv.URL)
	r.decl.Request.Finalize.Endpoint = srv.URL + "/buckets/{{ .external.current.json.id }}"

	_, err := r.reconcileMapped(context.Background(), observed(true, "200", "b-42"))
	require.NoError(t, err)
	require.Len(t, *seen, 1)
	assert.Equal(t, "DELETE /buckets/b-42 ", (*seen)[0], "the id the service assigned")
}
