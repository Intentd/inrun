// Package template is the context every Blueprint expression evaluates in.
// NewResolver starts from the CR (.metadata, .spec, .status) and the With
// methods add what a moment knows: .external, .children, .cross, .events,
// .metrics, .health, .args, .intent, the Blueprint's functions, sentinels and
// presets. Resolve evaluates an expression against it; webhooks, gates,
// resource templates, status fields, conversion and e2e expectations all go
// through it. Values without template syntax pass through.
//
// Package resources turns each kind's Blueprint template into its resolved
// form with a Resolver.
package template
