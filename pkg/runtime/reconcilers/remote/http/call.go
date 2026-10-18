package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote/contract"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// httpDoer is satisfied by *http.Client and any test double.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// remoteRequest is the JSON body POSTed to the remote endpoint: the CR's
// identity, the CR itself with request.object.exclude applied, the rendered
// reconcile.args, finalizing while the CR is being deleted and, when
// request.children is set, the CR's children. Nothing else from the runtime's
// prepared state is sent.
type remoteRequest struct {
	Key        string                 `json:"key"`
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Target     string                 `json:"target,omitempty"`
	Object     interface{}            `json:"object"`
	Args       map[string]interface{} `json:"args,omitempty"`
	Finalizing bool                   `json:"finalizing,omitempty"`
	Children   childrenMap            `json:"children,omitempty"`
}

// callRemote POSTs the reconcile request to the remote endpoint and returns the decoded result.
func (r *Reconciler) callRemote(ctx context.Context, req domain.Request) (contract.Result, error) {
	endpoint := r.resolveEndpoint(req)

	body, err := r.buildBody(req)
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: build request: %w", err)
	}

	method := http.MethodPost
	if r.decl.Request != nil && r.decl.Request.Method != "" {
		method = r.decl.Request.Method
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	headers, err := r.requestHeaders(req, method, endpoint, string(body))
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: request.%w", err)
	}
	for name, value := range headers {
		httpReq.Header.Set(name, value)
	}

	if err := r.applyAuth(ctx, httpReq); err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: auth: %w", err)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return contract.Result{}, fmt.Errorf("remote reconciler: POST %s: unexpected status %d", endpoint, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: read response: %w", err)
	}

	result, err := contract.Decode(data)
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: decode response: %w", err)
	}
	return result, nil
}

// resolverFor returns the request's template resolver.
func resolverFor(req domain.Request) (*template.Resolver, bool) {
	if req.Prepared == nil {
		return nil, false
	}
	resolver, ok := req.Prepared.Context.(*template.Resolver)
	return resolver, ok && resolver != nil
}

// resolveEndpoint evaluates the endpoint as a template when it contains "{{".
// Falls back to the static string on failure or when no resolver is available.
func (r *Reconciler) resolveEndpoint(req domain.Request) string {
	if req.Prepared == nil || req.Prepared.Context == nil {
		return r.decl.Endpoint
	}
	resolver, ok := req.Prepared.Context.(*template.Resolver)
	if !ok {
		return r.decl.Endpoint
	}
	eval := resolver.TemplateEvaluator()
	resolved, ok := eval(r.decl.Endpoint)
	if !ok {
		return r.decl.Endpoint
	}
	return resolved
}

// requestHeaders returns request.headers rendered against the CR, plus the
// idempotency key, for the contract call.
func (r *Reconciler) requestHeaders(req domain.Request, method, endpoint, body string) (map[string]string, error) {
	var spec types.CallSpec
	if r.decl.Request != nil {
		spec = r.decl.Request.CallSpec
	}
	render := func(v string) (string, error) { return v, nil }
	identity := req.Key
	if resolver, ok := resolverFor(req); ok {
		render = resolver.Resolve
		identity = external.ObjectIdentity(resolver.Data())
	}
	return external.RequestHeaders(render, spec, identity, method, endpoint, body)
}

// buildBody serialises the request for the remote endpoint: request.body
// rendered against the CR when declared, Inrun's request otherwise.
func (r *Reconciler) buildBody(req domain.Request) ([]byte, error) {
	if r.decl.Request != nil && r.decl.Request.Body != "" {
		resolver, ok := resolverFor(req)
		if !ok {
			return nil, fmt.Errorf("request.body: no template context for %s", req.Key)
		}
		rendered, err := resolver.Resolve(r.decl.Request.Body)
		if err != nil {
			return nil, fmt.Errorf("request.body: %w", err)
		}
		return []byte(rendered), nil
	}
	body := remoteRequest{
		Key:        req.Key,
		APIVersion: r.gvk.GroupVersion().String(),
		Kind:       r.gvk.Kind,
		Args:       req.Args(),
		Finalizing: req.Finalizing(),
	}
	if prep := req.Prepared; prep != nil {
		body.Target = prep.Target
		body.Object = prep.Object
		if r.decl.Request != nil && r.decl.Request.Object != nil && len(r.decl.Request.Object.Exclude) > 0 {
			body.Object = r.applyObjectExclusions(prep.Object, r.decl.Request.Object.Exclude)
		}
		body.Children = r.listChildren(prep.Object)
	}
	return json.Marshal(body)
}

// applyObjectExclusions returns a deep copy of obj with the given dot-notation paths removed.
func (r *Reconciler) applyObjectExclusions(obj domain.Object, paths []string) map[string]interface{} {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return domain.Raw(obj)
	}
	cp := u.DeepCopy().Object
	for _, path := range paths {
		structured.DeleteNestedPath(cp, path)
	}
	return cp
}

// effectiveAuth is the declared remote.auth with a secretRef without a
// namespace resolved in Inrun's own namespace; nil when no auth is declared.
func (r *Reconciler) effectiveAuth() *types.ExternalAuth {
	return r.inNamespace(r.decl.Auth)
}

// inNamespace returns auth with a secretRef without a namespace resolved in
// Inrun's own namespace.
func (r *Reconciler) inNamespace(auth *types.ExternalAuth) *types.ExternalAuth {
	if auth == nil || auth.SecretRef == nil || auth.SecretRef.Namespace != "" {
		return auth
	}
	cp := *auth.SecretRef
	cp.Namespace = r.namespace
	authCopy := *auth
	authCopy.SecretRef = &cp
	return &authCopy
}

// applyAuth injects the credential into the request header when auth is declared.
func (r *Reconciler) applyAuth(ctx context.Context, req *http.Request) error {
	auth := r.effectiveAuth()
	if auth == nil {
		return nil
	}
	credential, header, err := external.ResolveAuth(ctx, auth, r.kube.Clientset())
	if err != nil {
		return err
	}
	if credential != "" {
		req.Header.Set(header, "Bearer "+credential)
	}
	return nil
}
