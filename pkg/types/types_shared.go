// pkg/types/types_shared.go
package types

// ── Hook template source types — flat format ────────────────────────
//
// All template source types use a single flat field layout.
//
// Instead, Inrun uses:
//
//	Any string field containing "{{" → treated as a Go text/template expression.
//	                                    Evaluated against the live CR at reconcile time.
//	Any string field without "{{"    → treated as a static value. Used as-is.
//
// This means the same field can hold either a static value or a CR field reference
// without any additional YAML structure:
//
//	image: "nginx:latest"                 static — same for every CR of this type
//	image: "{{ .spec.image }}"            dynamic — resolved from CR spec at reconcile time
//	name:  "{{ .metadata.name }}-app"     dynamic — CR name with a static suffix
//	port:  "8080"                         static integer string
//	port:  "{{ .spec.port }}"             dynamic integer string
//
// Template context is the full CR object as map[string]interface{}:
//
//	.metadata.name        CR name
//	.metadata.namespace   CR namespace
//	.metadata.labels      CR labels map
//	.spec.*               any spec field (dynamic mode only — full spec accessible)
//	.status.*             any status field
//
// All resources created by hook templates receive owner references pointing to the CR.
// This means cascade deletion is automatic — child resources are garbage collected
// when the CR is deleted without requiring explicit onDelete declarations for most cases.

import (
	kwtypes "github.com/intentd/kubewright/core/types"
)

// ── Shared resource value types ───────────────────────────────────────────────

// Labels is kwtypes.Labels.
type Labels = kwtypes.Labels

// ResourceRequirements is kwtypes.ResourceRequirements.
type ResourceRequirements = kwtypes.ResourceRequirements

// EnvVar is kwtypes.EnvVar.
type EnvVar = kwtypes.EnvVar

// ValueFrom is kwtypes.ValueFrom.
type ValueFrom = kwtypes.ValueFrom

// EnvFromRef is kwtypes.EnvFromRef.
type EnvFromRef = kwtypes.EnvFromRef

// EnvFrom is kwtypes.EnvFrom.
type EnvFrom = kwtypes.EnvFrom

// SecretKeyRef is kwtypes.SecretKeyRef.
type SecretKeyRef = kwtypes.SecretKeyRef

// ConfigMapKeyRef is kwtypes.ConfigMapKeyRef.
type ConfigMapKeyRef = kwtypes.ConfigMapKeyRef
