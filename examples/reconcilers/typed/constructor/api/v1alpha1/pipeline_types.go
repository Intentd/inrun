//go:build ignore

// api/v1alpha1/pipeline_types.go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PipelinePhase represents the lifecycle phase of a Pipeline.
type PipelinePhase string

const (
	PipelinePhasePending   PipelinePhase = "Pending"
	PipelinePhaseRunning   PipelinePhase = "Running"
	PipelinePhaseSucceeded PipelinePhase = "Succeeded"
	PipelinePhaseFailed    PipelinePhase = "Failed"
)

// PipelineStep is one step in the pipeline.
type PipelineStep struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

// PipelineSpec defines the desired state of Pipeline.
type PipelineSpec struct {
	Image   string         `json:"image"`
	Steps   []PipelineStep `json:"steps"`
	Timeout string         `json:"timeout,omitempty"`
}

// PipelineStatus defines the observed state of Pipeline.
type PipelineStatus struct {
	Phase          PipelinePhase      `json:"phase,omitempty"`
	CurrentStep    string             `json:"currentStep,omitempty"`
	StartTime      *metav1.Time       `json:"startTime,omitempty"`
	CompletionTime *metav1.Time       `json:"completionTime,omitempty"`
	Message        string             `json:"message,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=`.status.currentStep`

// Pipeline is the Schema for the pipelines API.
type Pipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PipelineSpec   `json:"spec,omitempty"`
	Status            PipelineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PipelineList contains a list of Pipeline.
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}

var (
	Group    = "demo.inrun.dev"
	Kind     = "Pipeline"
	Version  = "v1alpha1"
	Resource = "pipelines"

	GroupVersion = schema.GroupVersion{
		Group:   Group,
		Version: Version,
	}
	GroupKind = schema.GroupKind{
		Group: Group,
		Kind:  Kind,
	}
	GroupVersionKind = schema.GroupVersionKind{
		Group:   Group,
		Version: Version,
		Kind:    Kind,
	}
	GroupVersionResource = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: Resource,
	}
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &Pipeline{}, &PipelineList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

func (p *Pipeline) DeepCopyObject() runtime.Object {
	if p == nil {
		return nil
	}
	out := new(Pipeline)
	p.DeepCopyInto(out)
	return out
}

func (p *Pipeline) DeepCopyInto(out *Pipeline) {
	*out = *p
	out.TypeMeta = p.TypeMeta
	p.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	p.Spec.DeepCopyInto(&out.Spec)
	p.Status.DeepCopyInto(&out.Status)
}

func (s *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *s
	if s.Steps != nil {
		out.Steps = make([]PipelineStep, len(s.Steps))
		for i := range s.Steps {
			step := s.Steps[i]
			step.Command = make([]string, len(s.Steps[i].Command))
			copy(step.Command, s.Steps[i].Command)
			out.Steps[i] = step
		}
	}
}

func (s *PipelineStatus) DeepCopyInto(out *PipelineStatus) {
	*out = *s
	if s.StartTime != nil {
		t := *s.StartTime
		out.StartTime = &t
	}
	if s.CompletionTime != nil {
		t := *s.CompletionTime
		out.CompletionTime = &t
	}
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		copy(out.Conditions, s.Conditions)
	}
}

func (pl *PipelineList) DeepCopyObject() runtime.Object {
	if pl == nil {
		return nil
	}
	out := new(PipelineList)
	*out = *pl
	if pl.Items != nil {
		out.Items = make([]Pipeline, len(pl.Items))
		for i := range pl.Items {
			pl.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}
