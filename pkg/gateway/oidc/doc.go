// Package oidc verifies the OIDC tokens callers present to the gateway. Cache
// fetches and caches each issuer's signing keys (JWKS), and Verify checks a
// token and returns its claims for the Blueprint's allow rules to match.
package oidc
