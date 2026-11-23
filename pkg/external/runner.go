package external

import (
	"context"
	"fmt"

	"github.com/intentd/tools/external"
	"github.com/intentd/tools/resolver"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/inrunmetrics"
)

// Templates is a template resolver that external results can be added to:
// Inrun's template.Resolver.
type Templates[R any] interface {
	Core() *resolver.Resolver
	WithExternal(results map[string]interface{}) R
}

// Run executes all declared external calls sequentially and returns a new
// resolver with results injected under .external.<name>.
//
// Calls run in declaration order. Each call can reference earlier calls'
// results in its own url:, query:, and body: template expressions.
//
// gvk is the CRD identifier used for metrics labelling and cache keying.
// cs is used to resolve auth.secretRef — pass nil when secretRef is not used.
// Returns the enriched resolver. The original is unchanged.
// Returns an error only when continueOnError is false and a call fails.
func Run[R Templates[R]](
	ctx context.Context,
	gvk string,
	resolver R,
	calls []types.ExternalCallSpec,
	cs kubernetes.Interface,
) (R, error) {
	if len(calls) == 0 {
		return resolver, nil
	}

	results := make(map[string]interface{}, len(calls))
	for i, call := range calls {
		entry, err := Call(ctx, gvk, fmt.Sprintf("external[%d]", i), resolver.Core(), call, cs)
		// Inject before any early return so status fields can reference this
		// call's result even when continueOnError is false.
		if entry != nil {
			results[call.Name] = entry
			resolver = resolver.WithExternal(results)
		}
		if err != nil {
			return resolver, err
		}
	}
	return resolver, nil
}

// Call makes one external call against tmpl and returns its result: the
// entry templates see as .external.<name>. where labels errors (for example
// "external[2]"). A call whose when/or conditions fail returns the skipped
// result. The error is set when the call must stop the caller: a template or
// auth that cannot be resolved, or a failure without continueOnError; the
// entry is still returned for a failed call so its error can be read.
func Call(
	ctx context.Context,
	gvk, where string,
	tmpl *resolver.Resolver,
	call types.ExternalCallSpec,
	cs kubernetes.Interface,
) (map[string]interface{}, error) {
	c := ToCall(call)
	if call.Auth != nil && types.EvaluateConditions(tmpl.Data(), call.When, call.Or, tmpl.Evaluator()) {
		credential, header, err := ResolveAuth(ctx, call.Auth, cs)
		if err != nil {
			return nil, fmt.Errorf("%s %q: %w", where, call.Name, err)
		}
		c.Credential, c.AuthHeader = credential, header
	}
	return external.Do(ctx, tmpl, c, external.Options{
		Where: where,
		Scope: gvk,
		Record: func(name, url string, durationSeconds float64, callErr string) {
			inrunmetrics.RecordExternalCall(gvk, name, url, durationSeconds, callErr, 0)
		},
	})
}

// ToCall maps an external call as a Blueprint or an e2e test declares it to
// the call tools/external makes. The credential is resolved separately
// (ResolveAuth); fires: and include: are Inrun's and decide whether and what
// to call, not how.
func ToCall(s types.ExternalCallSpec) external.Call {
	c := external.Call{
		Name:            s.Name,
		Protocol:        external.Protocol(s.Protocol),
		URL:             s.URL,
		Method:          s.Method,
		Headers:         s.Headers,
		Body:            s.Body,
		Query:           s.Query,
		ExpectedStatus:  s.ExpectedStatus,
		Timeout:         s.Timeout,
		CacheFor:        s.CacheFor,
		ContinueOnError: s.ContinueOnError,
		When:            s.When.Unwrap(),
		Or:              s.Or.Unwrap(),
		Idempotency:     toIdempotency(s.IdempotencyKey),
	}
	if s.Auth != nil {
		c.AuthHeader = s.Auth.Header
	}
	return c
}

func toIdempotency(k *types.IdempotencyKeyConfig) *external.Idempotency {
	if k == nil {
		return nil
	}
	return &external.Idempotency{Disabled: k.Disabled, Header: k.Header, Value: k.Value}
}
