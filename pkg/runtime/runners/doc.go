// Package runners applies resolved resource templates to the cluster, one
// Run* function per kind, creating, updating or deleting objects to match
// the CR.
//
// Most kinds share one loop, runChildren in child.go: each kind is a child
// description of how to read its template entries and which kubewright verbs
// write them. The descriptions are grouped as pkg/template/resources groups
// their builders: workloads, networking, config, policy, rbac and storage.
// Jobs, Secrets and custom resources have runners of their own.
package runners
