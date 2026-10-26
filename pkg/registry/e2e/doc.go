// Package e2e runs declarative end-to-end tests against a real cluster.
// From an e2e.yaml spec a Runner creates the cluster, applies CRDs, installs
// the operator, applies CRs, checks expectations and tears down.
// DiscoverE2EFiles finds specs recursively.
package e2e
