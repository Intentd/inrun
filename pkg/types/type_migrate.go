package types

import (
	"strings"

	"github.com/intentd/inrun/pkg/util/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CRMigrate is a CRD's migrate block. Selection scopes its migration at either
// level; To makes it kind level: each selected CR is rebuilt as a To CR (same
// name, namespace and spec) and server-side applied. Status is carried only
// with To.CarryStatus; the snapshot keeps it so rollback can restore it.
type CRMigrate struct {
	// To names the destination CRD. Leave it out to only scope field-level
	// (serve.fields migrate.from) migrations.
	To CRMigrateTarget `yaml:"to" json:"to"`

	// Selection restricts which CRs are migrated. Empty means all.
	Selection MigrateSelection `yaml:"selection,omitempty" json:"selection,omitempty"`

	// Prune deletes each source once its destination holds every source
	// field. Kind level only; the CLI asks first unless --yes.
	Prune bool `yaml:"prune,omitempty" json:"prune,omitempty"`
}

// ShouldPreserveMeta reports whether source labels and annotations are
// copied. Unset means true.
func (m CRMigrate) ShouldPreserveMeta() bool {
	return m.To.Metadata.PreserveSource == nil || *m.To.Metadata.PreserveSource
}

// GVK returns the destination GVK, or the zero value when malformed.
func (m CRMigrate) GVK() schema.GroupVersionKind {
	gvk, err := kube.GVKFromFields(m.To.APIVersion, m.To.Kind)
	if err != nil {
		return schema.GroupVersionKind{}
	}
	return gvk
}

// Group returns the destination API group, or "" when malformed.
func (m CRMigrate) Group() string {
	gv, err := schema.ParseGroupVersion(m.To.APIVersion)
	if err != nil {
		return ""
	}
	return gv.Group
}

// Plural returns the declared plural, else lowercased Kind + "s" (wrong for
// irregular plurals; validation warns).
func (m CRMigrate) Plural() string {
	if m.To.Plural != "" {
		return strings.ToLower(m.To.Plural)
	}
	return strings.ToLower(m.To.Kind) + "s"
}

// CRMigrateTarget is migrate.to.
type CRMigrateTarget struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	// Plural is used for RBAC; inferred from Kind when empty.
	Plural string `yaml:"plural,omitempty" json:"plural,omitempty"`
	// Namespace is used only when a cluster-scoped source moves to a
	// namespaced destination, and is required then.
	Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Metadata  CRMigrateMetadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// CarryStatus copies the source's status onto the destination. Off by
	// default: status is what the destination's controller has observed.
	CarryStatus bool `yaml:"carryStatus,omitempty" json:"carryStatus,omitempty"`

	// CRDFile fills APIVersion, Kind and Plural at load (declared values
	// must match) and lets validation check scope and schema.
	CRDFile string `yaml:"crdFile,omitempty" json:"crdFile,omitempty"`

	// ResolvedCRDFile is the absolute path read at load.
	ResolvedCRDFile string `yaml:"-" json:"-"`
}

// Declared reports whether migrate.to names a destination.
func (t CRMigrateTarget) Declared() bool {
	return t.APIVersion != "" || t.Kind != "" || t.CRDFile != ""
}

// MigrateSelection restricts which CRs a migration touches; every set field
// must match. Namespaces, Name and Selector filter the List; When and Or run
// per CR against the whole object. The CLI flags can narrow it, not widen it.
type MigrateSelection struct {
	// Namespaces is ignored for cluster-scoped CRDs.
	Namespaces []string `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	Name       string   `yaml:"name,omitempty" json:"name,omitempty"`
	// Selector is a Kubernetes label selector.
	Selector string `yaml:"selector,omitempty" json:"selector,omitempty"`
	// When must all pass; Or needs one to pass. CRs that fail are skipped.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`
	Or   []Condition `yaml:"or,omitempty" json:"or,omitempty"`
}

// HasConditions reports whether When or Or is set.
func (s MigrateSelection) HasConditions() bool {
	return len(s.When) > 0 || len(s.Or) > 0
}

// IsEmpty reports whether the selection selects every CR.
func (s MigrateSelection) IsEmpty() bool {
	return len(s.Namespaces) == 0 && s.Name == "" && s.Selector == "" && !s.HasConditions()
}

// CRMigrateMetadata is the destination's labels and annotations. Source
// labels and annotations are copied first (kubectl.kubernetes.io/* and
// inrun.dev/* dropped) unless PreserveSource is false; declared keys win.
type CRMigrateMetadata struct {
	// PreserveSource defaults to true.
	PreserveSource *bool             `yaml:"preserveSource,omitempty" json:"preserveSource,omitempty"`
	Labels         map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations    map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}
