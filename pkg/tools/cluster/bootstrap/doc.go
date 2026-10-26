// Package bootstrap gives the gateway least-privilege access to a remote
// cluster. Cluster creates a ServiceAccount, a ClusterRole scoped to the
// Blueprint's served CRDs, a binding and a token Secret on the target, then
// stores the credential where the gateway can use it. LoadConfig and
// ValidateConfig read the list of target clusters.
package bootstrap
