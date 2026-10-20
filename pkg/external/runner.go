package external

import (
	"context"
	"fmt"
	"net/http"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

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
func Run(
	ctx context.Context,
	gvk string,
	resolver *template.Resolver,
	calls []types.ExternalCallSpec,
	cs kubernetes.Interface,
) (*template.Resolver, error) {
	if len(calls) == 0 {
		return resolver, nil
	}

	results := make(map[string]interface{}, len(calls))
	for i, call := range calls {
		entry, err := Call(ctx, gvk, fmt.Sprintf("external[%d]", i), resolver, call, cs)
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

// Call makes one external call against resolver and returns its result: the
// entry templates see as .external.<name>. where labels errors (for example
// "external[2]"). A call whose when/or conditions fail returns the skipped
// result. The error is set when the call must stop the caller: a template or
// auth that cannot be resolved, or a failure without continueOnError; the
// entry is still returned for a failed call so its error can be read.
func Call(
	ctx context.Context,
	gvk, where string,
	resolver *template.Resolver,
	call types.ExternalCallSpec,
	cs kubernetes.Interface,
) (map[string]interface{}, error) {
	log := logger.FromContext(ctx)

	if !types.EvaluateConditions(resolver.Data(), call.When, call.Or, resolver.TemplateEvaluator()) {
		log.Debug().
			Str("call", call.Name).
			Str("where", where).
			Msg("external call skipped — when: conditions not met")
		return skippedResult(call.Protocol), nil
	}

	resolvedURL, err := resolver.Resolve(call.URL)
	if err != nil {
		return nil, fmt.Errorf("%s.url: %w", where, err)
	}
	resolvedQuery, err := resolver.Resolve(call.Query)
	if err != nil {
		return nil, fmt.Errorf("%s.query: %w", where, err)
	}
	resolvedBody, err := resolver.Resolve(call.Body)
	if err != nil {
		return nil, fmt.Errorf("%s.body: %w", where, err)
	}
	if call.Protocol == "" || call.Protocol == types.ProtocolHTTP || call.Protocol == types.ProtocolPrometheus {
		method := call.Method
		if method == "" {
			method = http.MethodGet
		}
		headers, err := RequestHeaders(resolver.Resolve, call.CallSpec, ObjectIdentity(resolver.Data()), method, resolvedURL, resolvedBody)
		if err != nil {
			return nil, fmt.Errorf("%s.%w", where, err)
		}
		call.Headers = headers // call is this function's copy
	}
	credential, _, err := ResolveAuth(ctx, call.Auth, cs)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", where, call.Name, err)
	}

	// Cache check for protocols that declare cacheFor:.
	var entry map[string]interface{}
	cacheHit := false
	if call.CacheFor != "" {
		key := cacheKey(gvk, call.Name, resolvedURL, resolvedQuery)
		if cached, ok := cacheGet(key); ok {
			entry = cached
			cacheHit = true
			log.Debug().Str("call", call.Name).Msg("external call: cache hit")
		}
	}

	if !cacheHit {
		client := newProtocolClient(call.Protocol)
		entry, err = client.Fetch(ctx, call, resolvedURL, resolvedQuery, resolvedBody, credential)
		if err != nil {
			// Hard error from the client (e.g. context cancelled).
			if !call.ContinueOnError {
				return nil, fmt.Errorf("external call %q: %w", call.Name, err)
			}
			log.Warn().Err(err).Str("call", call.Name).Msg("external call hard error — continuing")
			entry = errorResult(err.Error())
		}

		if call.CacheFor != "" {
			if ttl, err := core.ParseTimeDuration(call.CacheFor); err == nil {
				key := cacheKey(gvk, call.Name, resolvedURL, resolvedQuery)
				cacheSet(key, entry, ttl)
			}
		}
	}

	// Record metrics using the url for HTTP calls; query for others.
	callErr, _ := entry["error"].(string)
	statusStr, _ := entry["status"].(string)
	durationSeconds := 0.0 // non-HTTP clients don't surface duration yet
	metrics.RecordExternalCall(gvk, call.Name, resolvedURL, durationSeconds, callErr, 0)

	if callErr != "" {
		log.Warn().
			Str("call", call.Name).
			Str("url", resolvedURL).
			Str("error", callErr).
			Msg("external call failed")
		if !call.ContinueOnError {
			return entry, fmt.Errorf("external call %q failed: %s", call.Name, callErr)
		}
		return entry, nil
	}
	log.Debug().
		Str("call", call.Name).
		Str("url", resolvedURL).
		Str("status", statusStr).
		Msg("external call succeeded")
	return entry, nil
}

// skippedResult returns the zero-value result for a skipped call.
// HTTP uses status/body/error/called; non-HTTP uses result/raw/error/called.
func skippedResult(protocol types.ExternalProtocol) map[string]interface{} {
	switch protocol {
	case "", types.ProtocolHTTP:
		return map[string]interface{}{
			"status": "",
			"body":   "",
			"error":  "",
			"called": "false",
		}
	default:
		return map[string]interface{}{
			"result": "",
			"raw":    map[string]interface{}{},
			"error":  "",
			"called": "false",
		}
	}
}
