package types

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/kube"
	kwtypes "github.com/intentd/kubewright/core/types"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CustomResourceTemplateSource declares one arbitrary Kubernetes resource —
// any Kind not covered by one of Inrun's built-in resource types (a
// third-party CRD such as cert-manager's Certificate, or your own CRD).
// It is schema-agnostic: the spec block is written and applied exactly as
// declared in the Blueprint, with no built-in knowledge of its structure.
// Kubernetes enforces the resource's own CRD schema at apply time.
//
// Example:
//
//	onCreate:
//	  custom:
//	    - apiVersion: "cert-manager.io/v1"
//	      kind: Certificate
//	      metadata:
//	        name: "{{ .metadata.name }}-tls"
//	        namespace: "{{ .metadata.namespace }}"
//	      spec:
//	        secretName: "{{ .metadata.name }}-tls-secret"
//	        dnsNames:
//	          - "{{ .spec.hostname }}"
//	        issuerRef:
//	          name: letsencrypt-prod
//	          kind: ClusterIssuer
type CustomResourceTemplateSource struct {
	// APIVersion — the group/version of the target resource, e.g. "cert-manager.io/v1".
	// Required.
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`

	// Kind — the target resource's Kind, e.g. "Certificate". Required.
	Kind string `json:"kind" yaml:"kind"`

	// runtimeGVK is a cached GroupVersionKind derived from APIVersion + Kind.
	// It is intentionally not serialized to YAML/JSON.
	runtimeGVK *schema.GroupVersionKind `json:"-" yaml:"-"`

	// runtimeGVR is a cached GroupVersionResource resolved via RESTMapper.
	// It is intentionally not serialized to YAML/JSON.
	runtimeGVR *schema.GroupVersionResource `json:"-" yaml:"-"`

	// Metadata — name, namespace, labels, and annotations for the resource.
	// name is required. namespace is required for namespaced CRDs and must be
	// omitted for cluster-scoped ones; Inrun determines the correct scope
	// via discovery unless namespaced is set explicitly.
	//
	//	metadata:
	//	  name: "{{ .metadata.name }}-tls"
	//	  namespace: "{{ .metadata.namespace }}"
	//	  labels:
	//	    app: "{{ .metadata.name }}"
	Metadata CustomResourceMetadata `json:"metadata" yaml:"metadata"`

	// Spec is the conventional spec block for CRDs. It is schema-agnostic and
	// may contain templated values. Only template syntax is validated by
	// Inrun; structural/schema validation is deferred to the API server.
	Spec map[string]any `json:"spec,omitempty" yaml:"spec,omitempty"`

	// Other captures any top-level fields that are not spec/metadata.
	// This supports CRDs that place configuration at the top level instead of
	// under spec. This field is inlined during YAML/JSON unmarshalling.
	Other map[string]any `json:"-" yaml:",inline"`

	// CRDFile — the child kind's CRD manifest, relative to the Blueprint. At
	// load it fills apiVersion, kind and metadata.namespaced (declared values
	// must match), and inrun validate checks spec against the CRD's schema.
	CRDFile string `json:"crdFile,omitempty" yaml:"crdFile,omitempty"`

	// ResolvedCRDFile is the absolute path read at load.
	ResolvedCRDFile string `json:"-" yaml:"-"`

	// Observe — false: never read this object back into .children, saving
	// a read on every reconcile; the parent then can't see it. Default: read.
	Observe *bool `json:"observe,omitempty" yaml:"observe,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile" json:"reconcile,omitempty"`

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
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// CustomResourceMetadata is kwtypes.CustomResourceMetadata.
type CustomResourceMetadata = kwtypes.CustomResourceMetadata

// ShouldObserve reports whether this CustomResourceTemplateSource should be
// read back into .children during reconciliation. Defaults to true when
// Observe is nil; otherwise returns the declared value.
func (c *CustomResourceTemplateSource) ShouldObserve() bool {
	if c.Observe == nil {
		return true
	}
	return *c.Observe
}

// IsNamespaced returns whether the declaration intends the resource to be
// namespaced. It implements Inrun's defaulting rule: unspecified (nil)
// defaults to true (namespaced). Callers should still verify actual CRD scope
// via discovery before performing API operations.
func (c *CustomResourceTemplateSource) IsNamespaced() bool {
	if c == nil {
		return true // defensive default
	}

	if c.Metadata.Namespaced == nil {
		return true // default to namespaced for ergonomics
	}
	return *c.Metadata.Namespaced
}

// CustomResourceMeta returns the metadata block. It returns the concrete
// metadata struct rather than a pointer to encourage callers to treat the
// returned value as a snapshot. Mutations to the returned value do not affect
// the original CustomResource unless explicitly assigned back.
func (c *CustomResourceTemplateSource) CustomResourceMeta() CustomResourceMetadata {
	return c.Metadata
}

// HasLabels reports whether the resource has any labels after templating.
// Use this to decide whether to merge labels or to skip label-based selection.
func (c *CustomResourceTemplateSource) HasLabels() bool {
	return len(c.Metadata.Labels) > 0
}

// HasAnnotations reports whether the resource has any annotations after templating.
// Use this to decide whether to merge annotations or to skip annotation-based logic.
func (c *CustomResourceTemplateSource) HasAnnotations() bool {
	return len(c.Metadata.Annotations) > 0
}

// BuildGVK parses APIVersion and Kind and returns a GroupVersionKind.
func (c *CustomResourceTemplateSource) BuildGVK() (schema.GroupVersionKind, error) {
	if c == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("custom resource is nil")
	}
	gvk, err := kube.GVKFromFields(c.APIVersion, c.Kind)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	c.runtimeGVK = &gvk
	return gvk, nil
}

// RuntimeGVK returns the cached GVK if present, otherwise builds it.
func (c *CustomResourceTemplateSource) RuntimeGVK() (schema.GroupVersionKind, error) {
	if c == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("custom resource is nil")
	}
	if c.runtimeGVK != nil {
		return *c.runtimeGVK, nil
	}
	return c.BuildGVK()
}

// ResolveGVR resolves the GroupVersionResource for the CR's GVK using the
// provided RESTMapper and caches the result.
//
// Behaviour:
// - Builds the GVK if not already cached.
// - Calls mapper.RESTMapping(gvk.GroupKind(), gvk.Version).
// - Returns the resolved GVR or the underlying error from the mapper.
func (c *CustomResourceTemplateSource) ResolveGVR(mapper meta.RESTMapper) (schema.GroupVersionResource, error) {
	if c == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("custom resource is nil")
	}
	if mapper == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("restmapper is nil")
	}

	gvk, err := c.RuntimeGVK()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to resolve GVR for %s: %w", gvk.String(), err)
	}

	gvr := mapping.Resource
	c.runtimeGVR = &gvr
	return gvr, nil
}

// RuntimeGVR returns the cached GVR if present. If not cached, instructs the
// caller to call ResolveGVR with a RESTMapper first.
func (c *CustomResourceTemplateSource) RuntimeGVR() (schema.GroupVersionResource, error) {
	if c == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("custom resource is nil")
	}
	if c.runtimeGVR == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("gvr not resolved; call ResolveGVR(restMapper) first")
	}
	return *c.runtimeGVR, nil
}

// GVKString returns a human-friendly GVK string. Falls back to APIVersion/Kind
// if building the GVK fails.
func (c *CustomResourceTemplateSource) GVKString() string {
	if c == nil {
		return "<nil-custom-resource>"
	}
	if c.runtimeGVK != nil {
		return c.runtimeGVK.String()
	}
	// best-effort build
	if gvk, err := c.BuildGVK(); err == nil {
		return gvk.String()
	}
	return fmt.Sprintf("%s %s", c.APIVersion, c.Kind)
}

// GVRString returns a human-friendly GVR string if resolved, otherwise a placeholder.
func (c *CustomResourceTemplateSource) GVRString() string {
	if c == nil {
		return "<nil-custom-resource>"
	}
	if c.runtimeGVR != nil {
		return c.runtimeGVR.String()
	}
	return "<gvr-not-resolved>"
}
