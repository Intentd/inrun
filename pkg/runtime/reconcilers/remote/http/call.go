package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote/contract"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/utils"
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return contract.Result{}, fmt.Errorf("remote reconciler: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

// buildBody serialises the request for the remote endpoint.
func (r *Reconciler) buildBody(req domain.Request) ([]byte, error) {
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
		utils.DeleteNestedPath(cp, path)
	}
	return cp
}

// applyAuth injects the credential into the request header when auth is declared.
func (r *Reconciler) applyAuth(ctx context.Context, req *http.Request) error {
	if r.decl.Auth == nil {
		return nil
	}
	auth := r.decl.Auth
	if auth.SecretRef != nil && auth.SecretRef.Namespace == "" {
		cp := *auth.SecretRef
		cp.Namespace = r.namespace
		authCopy := *auth
		authCopy.SecretRef = &cp
		auth = &authCopy
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
