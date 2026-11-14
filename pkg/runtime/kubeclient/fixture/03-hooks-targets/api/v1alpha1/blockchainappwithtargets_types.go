package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type BlockchainAppWithTargetsSpec struct {
	Image      string `json:"image"`
	Network    string `json:"network"`
	NodeType   string `json:"nodeType,omitempty"`
	Replicas   int    `json:"replicas,omitempty"`
	ServiceUrl string `json:"serviceUrl,omitempty"`
}

type BlockchainAppWithTargetsStatus struct {
	Phase           string             `json:"phase,omitempty"`
	Network         string             `json:"network,omitempty"`
	NodeType        string             `json:"nodeType,omitempty"`
	FeatureEnabled  string             `json:"featureEnabled,omitempty"`
	InBusinessHours bool               `json:"inBusinessHours,omitempty"`
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
}

type BlockchainAppWithTargets struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BlockchainAppWithTargetsSpec   `json:"spec,omitempty"`
	Status            BlockchainAppWithTargetsStatus `json:"status,omitempty"`
}

type BlockchainAppWithTargetsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockchainAppWithTargets `json:"items"`
}

func (a *BlockchainAppWithTargets) DeepCopyObject() runtime.Object {
	if a == nil {
		return nil
	}
	out := new(BlockchainAppWithTargets)
	a.DeepCopyInto(out)
	return out
}

func (a *BlockchainAppWithTargets) DeepCopyInto(out *BlockchainAppWithTargets) {
	*out = *a
	out.TypeMeta = a.TypeMeta
	a.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = a.Spec
	a.Status.DeepCopyInto(&out.Status)
}

func (s *BlockchainAppWithTargetsStatus) DeepCopyInto(out *BlockchainAppWithTargetsStatus) {
	*out = *s
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		copy(out.Conditions, s.Conditions)
	}
}

func (al *BlockchainAppWithTargetsList) DeepCopyObject() runtime.Object {
	if al == nil {
		return nil
	}
	out := new(BlockchainAppWithTargetsList)
	*out = *al
	if al.Items != nil {
		out.Items = make([]BlockchainAppWithTargets, len(al.Items))
		for i := range al.Items {
			al.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}
