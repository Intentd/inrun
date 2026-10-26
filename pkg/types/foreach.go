// pkg/types/foreach.go
//
// ForEachSpec — declares dynamic resource expansion over a list field.
//
// When a template source declares forEach, the reconciler expands it into
// N resource declarations — one per element in the list field.
//
// The item is injected into the resolver context and available as:
//
//	{{ .item }}      — always available
//	{{ .<as> }}      — the name declared in forEach.as
//	{{ .index }}     — 0-based position in the list
//
// YAML:
//
//	onReconcile:
//	  deployments:
//	    - name: "{{ .metadata.name }}-{{ .item }}"
//	      image: "{{ .spec.image }}"
//	      forEach:
//	        field: spec.regions       # list field on the CR
//	        as: item                  # optional, default: "item"
//
// For CR with spec.regions: [us-east-1, eu-west-1, ap-southeast-1]:
// Produces three Deployments:
//
//	my-app-us-east-1
//	my-app-eu-west-1
//	my-app-ap-southeast-1
//
// Each declaration is fully independent — when:, or:, labels, and all
// other fields are evaluated per-item with .item in context.
//
// forEach works on all resource types: deployments, services, secrets,
// configmaps, jobs, cronjobs, serviceaccounts.
//
// Nesting: forEach is not nestable. One level of expansion per declaration.
// For matrix expansion (regions × environments), declare two separate
// resource types or use a hook.
package types

// ForEachSpec declares dynamic expansion over a list or map field.
type ForEachSpec struct {
	// Field is the dot-notation path to a list or map field on the CR.
	//
	// List field → one item per element:
	//   field: spec.regions  → ["us-east-1", "eu-west-1"]
	//   .item = element value ("us-east-1")
	//
	// Map field → one item per key (sorted alphabetically):
	//   field: spec.regions  → {us-east-1: {replicas: 3}, eu-west-1: {replicas: 1}}
	//   .item = map key ("us-east-1"), .value = map value ({replicas: 3})
	//   .value.replicas = "3"
	Field string `yaml:"field" json:"field"`

	// As is the name used to access the current item in template expressions.
	// Default: "item" — {{ .item }}
	// When set: both {{ .item }} and {{ .<as> }} work.
	//   as: region → {{ .region }} and {{ .item }} both resolve to the current element
	As string `yaml:"as,omitempty" json:"as,omitempty"`
}
