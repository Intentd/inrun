package types

// ServeView is the view: what the gateway returns to a caller in place of the
// raw CR (serve.view, or a serve target's view).
//
// Evaluated at request time against the CR already fetched from the API server.
// At apply time: the submitted CR (spec, metadata, labels, annotations only —
// children not yet created). At GET time: the full stored CR including status.
//
// The gateway does not fetch additional resources, call the runtime, or reason
// about Kubernetes internals. Its job remains narrow: authenticate, apply,
// fetch, delete, and — with a view — shape what the caller sees.
//
// Example:
//
//	serve:
//	  view:
//	    fullObject: true
//	    payload:
//	      phase:       '{{ .status.phase }}'
//	      serviceURL:  'https://{{ .metadata.name }}.{{ .spec.environment }}.myorg.io'
//	      nextSteps:   '{{ nextSteps }}'
//	    exclude:       '{{ toList (getAnnotation . "platform.myorg.io/exclude") }}'
type ServeView struct {
	// FullObject controls whether the full CR is included in the view before
	// payload and exclude are applied.
	//
	// true  (default) — start with the full CR; payload fields are merged in
	//                   on top, exclude paths are stripped afterward.
	// false           — start with an empty map; only payload fields appear.
	//
	// Omitting this field is equivalent to true — existing callers always
	// receive the full CR unless the platform team explicitly opts out.
	FullObject *bool `yaml:"fullObject,omitempty" json:"fullObject,omitempty"`

	// Payload is a map of named template expressions added to the response.
	//
	// Keys become top-level fields in the returned JSON object (alongside
	// apiVersion, kind, metadata, spec, status when fullObject: true, or as the
	// sole content when fullObject: false).
	//
	// Values are Go template expressions evaluated by the full Resolver with
	// the note FuncMap. Plain strings (no {{ }}) pass through unchanged.
	// Expressions that fail to resolve or produce "<no value>" are included
	// as empty strings — the caller can poll until values are populated.
	//
	// At apply time: .spec, .metadata, labels, and annotations are available.
	// At GET time:   .status is also available because the runtime has written it.
	//
	// Example:
	//   payload:
	//     phase:      '{{ .status.phase }}'
	//     serviceURL: 'https://{{ .metadata.name }}.myorg.io'
	//     nextSteps:  '{{ nextSteps }}'
	Payload map[string]string `yaml:"payload,omitempty" json:"payload,omitempty"`

	// Exclude is a list of dot-notation field paths to strip from the response
	// after payload is applied.
	//
	// Static declaration:
	//
	//	exclude:
	//	  - metadata.managedFields
	//	  - status.observedGeneration
	//
	// Dynamic via template expression in the list:
	//
	//	exclude:
	//	  - '{{ toList (getAnnotation . "platform.myorg.io/exclude") }}'
	//
	// inrun validate catches the case where a path appears in both payload and
	// exclude — exclude wins, but the conflict is surfaced as a warning.
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`

	// Poll configures how the pollUrl in the Gateway API response is generated.
	// When omitted, the default derived pollUrl is used.
	// See ServePollingConfig for override options.
	Poll *ServePollingConfig `yaml:"poll,omitempty" json:"poll,omitempty"`
}

// ServePollingConfig configures how the poll URL is generated for the Gateway API response.
//
// By default, PollURL is derived from the resource's kind, namespace, and name:
//
//	/api/v1/resources/{kind}/{namespace}/{name}
//
// Two overrides are available:
//   - field: append ?field=<value> to the resolved poll URL for lightweight polling
//   - url:   replace the default poll URL entirely with a custom template
//
// When both are set, url replaces the default, and field is appended to it.
// When only field is set, it is appended to the default poll URL.
//
// Examples:
//
//	poll:
//	  field: status.phase
//	  → /api/v1/resources/App/default/my-app?field=status.phase
//
//	poll:
//	  url: '/api/v2/resources/{{ .kind }}/{{ .namespace }}/{{ .name }}'
//	  field: status.phase
//	  → /api/v2/resources/App/default/my-app?field=status.phase
//
//	poll:
//	  url: 'https://monitor.myorg.io/status/{{ .metadata.name }}'
//	  field: ready
//	  → https://monitor.myorg.io/status/my-app?field=ready
type ServePollingConfig struct {
	// URL is the polling endpoint template.
	// When set, it completely replaces the default derived pollUrl.
	// Template expressions are evaluated against the CR at request time.
	// Example: '/api/v2/resources/{{ .kind }}/{{ .namespace }}/{{ .name }}'
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Field is a shortcut for appending ?field=<value> to the default pollUrl.
	// When set, the default pollUrl is used with ?field= appended.
	// Example: 'status.phase' → /api/v1/resources/App/default/my-app?field=status.phase
	Field string `yaml:"field,omitempty" json:"field,omitempty"`
}

// GetServePollingURL returns the polling URL for the Gateway API response for this CRD.
func (c *CRDEntry) GetServePollingURL() string {
	if !c.HasServePollingConfig() {
		return ""
	}
	return c.Serve.View.Poll.URL
}

// GetServePollingField returns the polling field for the Gateway API response for this CRD.
func (c *CRDEntry) GetServePollingField() string {
	if !c.HasServePollingConfig() {
		return ""
	}
	return c.Serve.View.Poll.Field
}

// GetServePollingConfig returns the polling config for the Gateway API response for this CRD.
func (c *CRDEntry) GetServePollingConfig() *ServePollingConfig {
	if !c.HasServePollingConfig() {
		return nil
	}
	return c.Serve.View.Poll
}

// HasServePollingConfig reports whether the polling config is declared.
func (c *CRDEntry) HasServePollingConfig() bool {
	return c.Serve != nil && c.Serve.View != nil && c.Serve.View.Poll != nil
}

// IncludesFullObject reports whether the full CR is the view's starting
// point: fullObject omitted or true.
func (r *ServeView) IncludesFullObject() bool {
	return r.FullObject == nil || *r.FullObject
}

// HasPayload reports whether any payload expressions are declared.
func (r *ServeView) HasPayload() bool {
	return len(r.Payload) > 0
}

// HasExclude reports whether an exclude expression is declared.
func (r *ServeView) HasExclude() bool {
	return len(r.Exclude) > 0
}

// HasPoll reports whether the polling config is declared.
func (r *ServeView) HasPoll() bool {
	return r.Poll != nil
}
