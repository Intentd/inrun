// Package api is the gateway's HTTP API for served CRDs. Callers apply an
// intent or a full CR, read and list resources, delete them, and fetch the
// schema of what a CRD accepts, all without a kubeconfig. Tokens declared in
// the Blueprint decide who may do what, in which namespaces and clusters.
package api
