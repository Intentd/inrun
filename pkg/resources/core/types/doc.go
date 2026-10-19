// Package types holds the field types a resource spec is made of: env and
// envFrom, resource requirements, probes, pod and container security,
// volumes, rolling update, HPA behavior, network policy rules, limit range
// items, RBAC rules, labels and custom resource metadata. Each kind's Spec
// in pkg/resources uses them; Inrun's Blueprint types alias them, so a
// Blueprint and a Spec describe a field the same way.
package types
