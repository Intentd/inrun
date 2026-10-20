package types

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// CallSpec is one HTTP call: what an external call (reconcile.external) and
// the remote reconciler's request and finalize calls have in common. Both
// embed it, so a field added here reaches every call.
type CallSpec struct {
	// URL is the endpoint or connection string. Template expressions supported.
	//   url: "{{ .spec.serviceUrl }}/health"
	//   url: "redis://redis-svc:6379"
	// For the remote request and finalize: default remote.endpoint.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Method is the HTTP method. Default: GET for external calls, POST for
	// the remote request. HTTP only.
	Method string `yaml:"method,omitempty" json:"method,omitempty"`

	// Headers are additional HTTP headers. Values are templates, like url and
	// body; names are literal. Credentials belong in auth, not here. HTTP only.
	//   headers:
	//     X-Tenant: "{{ .metadata.namespace }}"
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Body is the request body for POST/PUT/PATCH. Template expressions supported.
	//   body: '{"name": "{{ .metadata.name }}"}'
	// HTTP only.
	Body string `yaml:"body,omitempty" json:"body,omitempty"`

	// ExpectedStatus lists the HTTP status codes that count as success, e.g.
	// [200, 404] for a read where "not found" is an answer. HTTP only.
	// Default: any status below 400.
	ExpectedStatus []int `yaml:"expectedStatus,omitempty" json:"expectedStatus,omitempty"`

	// When and Or decide whether the call is made this pass.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`
	Or   []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// Timeout is the maximum duration for this call. Format: "5s", "1m",
	// "500ms". Default: 10s for external calls, remote.timeout for the
	// remote request.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Auth declares the credential for this call. For the remote request:
	// default remote.auth.
	Auth *ExternalAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// RetryBackoff configures how many times and how long to wait between
	// retries before returning an error. Shorthand ("3s") sets initial only;
	// full form gives full control.
	RetryBackoff *RetryBackoffConfig `yaml:"retryBackoff,omitempty" json:"retryBackoff,omitempty"`

	// IdempotencyKey controls the key Inrun sends on POST and PATCH so a
	// retried write is not applied twice. Absent: sent as Idempotency-Key with
	// a key derived from the CR's UID and the request. HTTP only.
	//   idempotencyKey: false
	//   idempotencyKey: {header: PayPal-Request-Id}
	IdempotencyKey *IdempotencyKeyConfig `yaml:"idempotencyKey,omitempty" json:"idempotencyKey,omitempty"`
}

// DefaultIdempotencyHeader is the header the idempotency key is sent in,
// the name in the IETF HTTP API working group's draft.
const DefaultIdempotencyHeader = "Idempotency-Key"

// IdempotencyKeyConfig overrides the idempotency key Inrun sends on POST and
// PATCH. In YAML it is false (send none) or an object.
type IdempotencyKeyConfig struct {
	// Disabled sends no key: idempotencyKey: false.
	Disabled bool `yaml:"-" json:"-"`
	// Header is the header name. Default: Idempotency-Key.
	Header string `yaml:"header,omitempty" json:"header,omitempty"`
	// Value is a template for the key, for an API with its own key format.
	// Default: derived from the CR's UID, the method, the URL and the body.
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

// EffectiveHeader returns the header name, defaulted.
func (c *IdempotencyKeyConfig) EffectiveHeader() string {
	if c == nil || c.Header == "" {
		return DefaultIdempotencyHeader
	}
	return c.Header
}

// IsDisabled reports whether no key is sent.
func (c *IdempotencyKeyConfig) IsDisabled() bool { return c != nil && c.Disabled }

type idempotencyKeyFields struct {
	Header string `yaml:"header,omitempty" json:"header,omitempty"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
}

// UnmarshalYAML accepts false (no key) or {header, value}.
func (c *IdempotencyKeyConfig) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var b bool
		if err := n.Decode(&b); err != nil || b {
			return fmt.Errorf("line %d: idempotencyKey must be false or an object with header and value", n.Line)
		}
		*c = IdempotencyKeyConfig{Disabled: true}
		return nil
	}
	var f idempotencyKeyFields
	if err := n.Decode(&f); err != nil {
		return err
	}
	*c = IdempotencyKeyConfig{Header: f.Header, Value: f.Value}
	return nil
}

// MarshalYAML writes false for a disabled key.
func (c IdempotencyKeyConfig) MarshalYAML() (interface{}, error) {
	if c.Disabled {
		return false, nil
	}
	return idempotencyKeyFields{Header: c.Header, Value: c.Value}, nil
}

// UnmarshalJSON accepts false (no key) or {header, value}.
func (c *IdempotencyKeyConfig) UnmarshalJSON(b []byte) error {
	if string(b) == "false" {
		*c = IdempotencyKeyConfig{Disabled: true}
		return nil
	}
	var f idempotencyKeyFields
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("idempotencyKey must be false or an object with header and value: %w", err)
	}
	*c = IdempotencyKeyConfig{Header: f.Header, Value: f.Value}
	return nil
}

// MarshalJSON writes false for a disabled key.
func (c IdempotencyKeyConfig) MarshalJSON() ([]byte, error) {
	if c.Disabled {
		return []byte("false"), nil
	}
	return json.Marshal(idempotencyKeyFields{Header: c.Header, Value: c.Value})
}
