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
	e := blueprintWithRemote("https://buckets.internal/buckets/{{ .metadata.name }}", types.RemoteReconcileProtocolHTTP, remoteResources())
	remote := crdPtr(e, "app").Box().Reconcile.Remote
	remote.Request = &types.RemoteRequestConfig{CallSpec: types.CallSpec{Method: "PUT",
		Body:           `{"quotaGB": {{ .spec.quotaGB }}}`,
		ExpectedStatus: []int{200}}, Finalize: &types.CallSpec{Method: "DELETE", ExpectedStatus: []int{202, 404}},
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
	assert.ErrorContains(t, err, "request: expectedStatus 700 is not an HTTP status code")
	err = mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) { r.Request.Finalize.ExpectedStatus = []int{99} })
	assert.ErrorContains(t, err, "request.finalize: expectedStatus 99 is not an HTTP status code")
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
		r.Request.Conditions = []types.Condition{{Field: "external.current.status", Equals: "404"}}
		r.Request.Finalize.URL = "https://buckets.internal/buckets/{{ .external.current.json.id }}"
		r.Request.Finalize.Or = []types.Condition{{Field: "external.current.status", Equals: "200"}}
	}))

	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Finalize.Conditions = []types.Condition{{Field: "x", Min: "1", Max: "2"}}
	})
	assert.ErrorContains(t, err, "request.finalize condition on x declares min and max")
}

func TestValidateRemoteMapping_RequestWhenNeedsResponse(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Response = nil
		r.Request.Finalize = nil
		r.Request.ExpectedStatus = nil
		r.Request.Conditions = []types.Condition{{Field: "spec.enabled", Equals: "true"}}
	})
	assert.ErrorContains(t, err, "request.when/or needs reconcile.remote.response")
}

func TestValidateRemoteMapping_HeadersAndKey(t *testing.T) {
	require.NoError(t, mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Headers = map[string]string{"X-Tenant": "{{ .metadata.namespace }}"}
		r.Request.IdempotencyKey = &types.IdempotencyKeyConfig{Header: "PayPal-Request-Id"}
		r.Request.Finalize.URL = "https://buckets.internal/buckets/{{ .status.bucketId }}"
	}))

	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Headers = map[string]string{"Bad Name": "x"}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "header name")

	err = mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Headers = map[string]string{"X-Tenant": "{{ .metadata.namespace "}
	})
	require.Error(t, err, "header values are checked as templates")
}

func TestValidateRemoteMapping_RetryBackoffDoesNotApply(t *testing.T) {
	err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
		r.Request.Finalize.RetryBackoff = &types.RetryBackoffConfig{}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request.finalize.retryBackoff")
}

func TestValidateRemoteMapping_ContractIgnoresCallFields(t *testing.T) {
	for _, edit := range []func(*types.RemoteRequestConfig){
		func(rq *types.RemoteRequestConfig) { rq.URL = "https://other" },
		func(rq *types.RemoteRequestConfig) { rq.Timeout = "5s" },
		func(rq *types.RemoteRequestConfig) {
			rq.Auth = &types.ExternalAuth{Env: "TOKEN"}
		},
	} {
		err := mappedRemote(t, func(r *types.RemoteReconcilerDeclaration) {
			r.Response = nil
			r.Request.ExpectedStatus = nil
			r.Request.Finalize = nil
			edit(r.Request)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "needs reconcile.remote.response")
	}
}

func TestValidateRemoteMapping_Warnings(t *testing.T) {
	e := blueprintWithRemote("https://buckets.internal/buckets", types.RemoteReconcileProtocolHTTP, remoteResources())
	remote := crdPtr(e, "app").Box().Reconcile.Remote
	remote.Request = &types.RemoteRequestConfig{CallSpec: types.CallSpec{Method: "PUT",
		Headers:        map[string]string{"Authorization": "Bearer x"},
		IdempotencyKey: &types.IdempotencyKeyConfig{Header: "X-Key"}}}
	remote.Response = &types.RemoteResponseConfig{Requeue: &types.RequeueConfig{After: "10s"}}
	crd := crdPtr(e, "app")
	require.NoError(t, e.validateRemote("app", crd))
	w := crd.Warnings
	require.True(t, w.HasWarnings())
	assert.Contains(t, w.String(), "Authorization")
	assert.Contains(t, w.String(), "only on POST and PATCH")
}
