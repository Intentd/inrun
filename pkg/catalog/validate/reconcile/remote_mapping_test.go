package reconcile

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mappedRemote is a valid mapped remote; each test breaks one thing.
func mappedRemote(t *testing.T, edit func(*types.RemoteReconcilerDeclaration)) error {
	t.Helper()
	e := catalogWithRemote("https://buckets.internal/buckets/{{ .metadata.name }}", types.RemoteReconcileProtocolHTTP, remoteResources())
	remote := crdPtr(e, "app").Box().Reconcile.Remote
	remote.Request = &types.RemoteRequestConfig{
		Method:         "PUT",
		Body:           `{"quotaGB": {{ .spec.quotaGB }}}`,
		ExpectedStatus: []int{200},
		Finalize:       &types.RemoteRequestCall{Method: "DELETE", ExpectedStatus: []int{202, 404}},
	}
	remote.Response = &types.RemoteResponseConfig{
		Status:  &types.StatusConfig{Fields: []types.StatusFieldSpec{{Path: "state", Value: "{{ .response.json.status }}"}}},
		Requeue: &types.RequeueConfig{After: "10s"},
	}
	if edit != nil {
		edit(remote)
	}
	return e.validateRemote("app", crdPtr(e, "app"))
}

func TestValidateRemoteMapping_Valid(t *testing.T) {
	require.NoError(t, mappedRemote(t, nil))
}

func TestValidateRemoteMapping_FinalizeNeedsResponse(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Response = nil
		r.Request.ExpectedStatus = nil
	})
	assert.ErrorContains(t, err, "request.finalize needs reconcile.remote.response")
}

func TestValidateRemoteMapping_BadMethod(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) { r.Request.Method = "FETCH" })
	assert.ErrorContains(t, err, `"FETCH" is not a valid HTTP method`)
}

func TestValidateRemoteMapping_StatusCodeRange(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) { r.Request.ExpectedStatus = []int{200, 700} })
	assert.ErrorContains(t, err, "request.expectedStatus 700 is not an HTTP status code")
	err = mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) { r.Request.Finalize.ExpectedStatus = []int{99} })
	assert.ErrorContains(t, err, "request.finalize.expectedStatus 99 is not an HTTP status code")
}

func TestValidateRemoteMapping_ExpectedStatusNeedsResponse(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Response = nil
		r.Request.Finalize = nil
	})
	assert.ErrorContains(t, err, "request.expectedStatus needs reconcile.remote.response")
}

func TestValidateRemoteMapping_StatusFieldNeedsPath(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Response.Status.Fields = []types.StatusFieldSpec{{Value: "x"}}
	})
	assert.ErrorContains(t, err, "response.status.fields[0].path is required")
}

func TestValidateRemoteMapping_RequeueAfter(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) { r.Response.Requeue.After = "soon" })
	assert.ErrorContains(t, err, "response.requeue.after")
}

func TestValidateRemoteMapping_CallConditions(t *testing.T) {
	require.NoError(t, mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.When = []types.Condition{{Field: "external.current.status", Equals: "404"}}
		r.Request.Finalize.Endpoint = "https://buckets.internal/buckets/{{ .external.current.json.id }}"
		r.Request.Finalize.Or = []types.Condition{{Field: "external.current.status", Equals: "200"}}
	}))

	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Finalize.When = []types.Condition{{Field: "x", Min: "1", Max: "2"}}
	})
	assert.ErrorContains(t, err, "request.finalize condition on x declares min and max")
}

func TestValidateRemoteMapping_RequestWhenNeedsResponse(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Response = nil
		r.Request.Finalize = nil
		r.Request.ExpectedStatus = nil
		r.Request.When = []types.Condition{{Field: "spec.enabled", Equals: "true"}}
	})
	assert.ErrorContains(t, err, "request.when/or needs reconcile.remote.response")
}
