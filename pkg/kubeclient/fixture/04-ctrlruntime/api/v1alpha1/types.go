package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// WebAppSpec defines the desired state of WebApp.
type WebAppSpec struct {
	// Image is the container image to deploy.
	Image string `json:"image"`

	// Replicas is the desired number of pods. Defaults to 1.
	// +optional
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Port is the container port to expose. Defaults to 80.
	// +optional
	// +kubebuilder:default=80
	Port int32 `json:"port,omitempty"`
}

// WebAppStatus defines the observed state of WebApp.
type WebAppStatus struct {
	Phase    string `json:"phase,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Replicas int32  `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WebApp is the Schema for the webapps API.
type WebApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebAppSpec   `json:"spec,omitempty"`
	Status WebAppStatus `json:"status,omitempty"`
}

func (in *WebApp) DeepCopyObject() runtime.Object {
	out := new(WebApp)
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return out
}

// +kubebuilder:object:root=true

// WebAppList contains a list of WebApp.
type WebAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebApp `json:"items"`
}

func (in *WebAppList) DeepCopyObject() runtime.Object {
	out := new(WebAppList)
	*out = *in
	if in.Items != nil {
		out.Items = make([]WebApp, len(in.Items))
		for i := range in.Items {
			out.Items[i] = *in.Items[i].DeepCopyObject().(*WebApp)
		}
	}
	return out
}

var GroupVersionKind = schema.GroupVersionKind{
	Group:   "migration.demo.inrun.dev",
	Version: "v1alpha1",
	Kind:    "WebApp",
}

var SchemeGroupVersion = schema.GroupVersion{
	Group:   "migration.demo.inrun.dev",
	Version: "v1alpha1",
}

func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion, &WebApp{}, &WebAppList{})
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}
