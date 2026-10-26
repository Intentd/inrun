// Package registration builds, applies and removes the webhook configurations
// that point the API server at the gateway: validation, mutation, deletion,
// namespace and strict-mode protection. Every function is idempotent, so the
// webhook server calls them once at startup and the housekeeper calls them
// whenever a configuration drifts or disappears.
package registration
