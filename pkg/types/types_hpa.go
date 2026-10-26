// pkg/types/types_hpa.go
package types

import kwtypes "github.com/intentd/kubewright/core/types"

// ── HorizontalPodAutoscaler ───────────────────────────────────────────────────

// ScaleTargetRef is kwtypes.ScaleTargetRef.
type ScaleTargetRef = kwtypes.ScaleTargetRef

// HPATemplateSource declares one HorizontalPodAutoscaler to be managed by Inrun.
//
// Example:
//
//	onReconcile:
//	  hpa:
//	    - name: "{{ .metadata.name }}-hpa"
//	      deploymentRef: "{{ .metadata.name }}"
//	      minReplicas: "{{ .spec.minReplicas }}"
//	      maxReplicas: "{{ .spec.maxReplicas }}"
//	      targetCPUUtilizationPercentage: "80"
//	      forEach:
//	        field: spec.services
//	        as: item
type HPATemplateSource struct {
	// Name — HPA resource name. Default: "{{ .metadata.name }}-hpa"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Namespace — target namespace. Default: CR namespace.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// ScaleTargetRef — the target workload this HPA scales.
	// Supports Deployment, ReplicaSet, StatefulSet, or any scalable resource.
	// Supports template expressions: "{{ .metadata.name }}"
	//
	//	scaleTargetRef:
	//	  apiVersion: apps/v1
	//	  kind: Deployment
	//	  name: "{{ .metadata.name }}-deployment"
	ScaleTargetRef ScaleTargetRef `yaml:"scaleTargetRef,omitempty" json:"scaleTargetRef,omitempty"`

	// MinReplicas — minimum replica count as a string. Supports template expressions.
	MinReplicas string `yaml:"minReplicas,omitempty" json:"minReplicas,omitempty"`

	// MaxReplicas — maximum replica count as a string. Supports template expressions.
	MaxReplicas string `yaml:"maxReplicas,omitempty" json:"maxReplicas,omitempty"`

	// TargetCPUUtilizationPercentage — CPU utilization target (0-100). Supports templates.
	// When behavior.preset is set and this field is empty, the preset provides the default.
	TargetCPUUtilizationPercentage string `yaml:"targetCPUUtilizationPercentage,omitempty" json:"targetCPUUtilizationPercentage,omitempty"`

	// Behavior — scale-up and scale-down behavior configuration.
	// Set preset for a complete preset, or configure scaleUp/scaleDown explicitly.
	// Preset and explicit fields are mutually exclusive.
	//
	//	behavior:
	//	  preset: web
	//
	//	behavior:
	//	  scaleUp:
	//	    stabilizationWindowSeconds: 0
	//	  scaleDown:
	//	    stabilizationWindowSeconds: 300
	Behavior *HPABehavior `yaml:"behavior,omitempty" json:"behavior,omitempty"`

	// Labels applied to HPA metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.services
	//	  as: item
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
