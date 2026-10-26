// api/v1alpha1/resource_probe_types.go
//
// Typed Go struct for the ResourceProbe CRD (group: resources.inrun.dev).
// Gives type-safe spec access in resource_hooks.go instead of unstructured map navigation.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceProbeSpec is the desired state of a ResourceProbe.
// One field per resource dimension exercised by the hook.
type ResourceProbeSpec struct {
	// Image — container image used by all workload resources.
	Image string `json:"image"`

	// Replicas — replica count for Deployment, StatefulSet, and ReplicaSet.
	Replicas string `json:"replicas"`

	// Port — container port for all workload resources.
	Port string `json:"port"`

	// Storage — PV / PVC size (e.g. "1Gi").
	Storage string `json:"storage"`

	// Schedule — CronJob schedule expression (e.g. "*/5 * * * *").
	Schedule string `json:"schedule"`
}

// ResourceProbeStatus defines the observed state of ResourceProbe.
type ResourceProbeStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ResourceProbe is the Schema for the resourceprobes API.
type ResourceProbe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceProbeSpec   `json:"spec,omitempty"`
	Status ResourceProbeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceProbeList contains a list of ResourceProbe.
type ResourceProbeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceProbe `json:"items"`
}

var (
	GroupVersion  = schema.GroupVersion{Group: "resources.inrun.dev", Version: "v1alpha1"}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ResourceProbe{},
		&ResourceProbeList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

// DeepCopyObject implements runtime.Object.
func (r *ResourceProbe) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(ResourceProbe)
	r.DeepCopyInto(out)
	return out
}

func (r *ResourceProbe) DeepCopyInto(out *ResourceProbe) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = r.Spec
	r.Status.DeepCopyInto(&out.Status)
}

func (s *ResourceProbeStatus) DeepCopyInto(out *ResourceProbeStatus) {
	*out = *s
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		copy(out.Conditions, s.Conditions)
	}
}

func (rl *ResourceProbeList) DeepCopyObject() runtime.Object {
	if rl == nil {
		return nil
	}
	out := new(ResourceProbeList)
	*out = *rl
	if rl.Items != nil {
		out.Items = make([]ResourceProbe, len(rl.Items))
		for i := range rl.Items {
			rl.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}
