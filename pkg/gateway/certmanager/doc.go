// Package certmanager manages the TLS certificate for the gateway's webhook
// server: it generates a self-signed certificate when none is configured,
// stores it in a Secret, and renews it before it expires.
package certmanager
