// Package admission serves the gateway's /validate and /mutate webhooks. The
// API server sends each CREATE and UPDATE of a CRD with admission rules: Mutate
// applies the Blueprint's mutation rules as a JSON Patch, and Validate denies
// the request when a validation rule fails. Mutation runs first, so validation
// sees the defaults.
package admission
