package types

import (
	"context"

	"github.com/intentd/tools/assert"
	"github.com/intentd/tools/duration"
	"github.com/intentd/tools/kube"
	"github.com/intentd/tools/resolver/env"
	"github.com/intentd/tools/retry"
	"github.com/intentd/tools/term"
)

type (
	TLSSpec = kube.TLSSpec
	// Duration is a time.Duration that unmarshals from YAML strings — Go's units
	// ("15s", "2m", "1h") plus d/w/mo/y ("10d", "2w", "3mo", "1y").
	Duration = duration.Duration
	// BackoffConfig is declared under queue.retryBackoff or external[].retryBackoff.
	// Shorthand: a plain duration string sets Initial only.
	// Full form: set Initial, Max, Multiplier, and MaxAttempts individually.
	RetryBackoffConfig = retry.BackoffConfig

	// Info holds non‑fatal validation messages for this CRD.
	// Populated during Blueprint validation (e.g., enrichments).
	Info = term.InfoMessages

	// HasInfo returns true if there are any informational messages.
	// Warnings holds non‑fatal validation messages for this CRD.
	// Populated during Blueprint validation (e.g., enrichment, deletion protection overrides).
	Warnings = term.WarningMessages
)

// The assertion vocabulary lives in tools/assert; these names keep Inrun's
// e2e and simulate types reading as before.
type (
	Assertions     = assert.Assertions
	TextAssertion  = assert.TextAssertion
	TextAssertions = assert.TextAssertions
	JSONAssertion  = assert.JSONAssertion
	JSONAssertions = assert.JSONAssertions
	Extractor      = assert.Extractor
	ExtractRequest = assert.ExtractRequest
)

// AssertionNames returns the assertion fields a check can declare.
func AssertionNames() []string { return assert.AssertionNames() }

// ApplyExtract runs a step's extractor over its output.
func ApplyExtract(ctx context.Context, req ExtractRequest) (string, error) {
	return assert.ApplyExtract(ctx, req)
}

// EnvResolver resolves ${NAME} for one expectation: its env, then the spec's,
// then the process environment (tools/resolver/env). Built by
// E2ESpec.ResolveEnv.
type EnvResolver = env.Resolver

// NoopEnvResolver returns an EnvResolver that declares nothing: Lookup falls
// through to the process environment, and Pairs returns nil.
func NoopEnvResolver() EnvResolver { return env.Resolver{} }

// ExpandEnv returns a copy of v with ${NAME} expanded through e in every
// string it holds; fields tagged env:"-" keep their text.
func ExpandEnv[T any](e EnvResolver, v T) T { return env.ExpandAll(e, v) }

// UnknownEnv returns the ${NAME} references in v that e cannot resolve.
func UnknownEnv(e EnvResolver, v any) []string { return env.Unknown(e, v) }
