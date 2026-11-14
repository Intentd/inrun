package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type BlockchainNodeSpec struct {
	Image      string `json:"image"`
	Network    string `json:"network"`
	Replicas   int    `json:"replicas,omitempty"`
	ServiceUrl string `json:"serviceUrl,omitempty"`
}

type BlockchainNodeStatus struct {
	Phase           string             `json:"phase,omitempty"`
	Network         string             `json:"network,omitempty"`
	Replicas        int32              `json:"replicas,omitempty"`
	FeatureEnabled  string             `json:"featureEnabled,omitempty"`
	InBusinessHours bool               `json:"inBusinessHours,omitempty"`
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
}

type BlockchainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BlockchainNodeSpec   `json:"spec,omitempty"`
	Status            BlockchainNodeStatus `json:"status,omitempty"`
}

type BlockchainNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockchainNode `json:"items"`
}

func (n *BlockchainNode) DeepCopyObject() runtime.Object {
	if n == nil {
		return nil
	}
	out := new(BlockchainNode)
	n.DeepCopyInto(out)
	return out
}

func (n *BlockchainNode) DeepCopyInto(out *BlockchainNode) {
	*out = *n
	out.TypeMeta = n.TypeMeta
	n.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = n.Spec
	n.Status.DeepCopyInto(&out.Status)
}

func (s *BlockchainNodeStatus) DeepCopyInto(out *BlockchainNodeStatus) {
	*out = *s
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		copy(out.Conditions, s.Conditions)
	}
}

func (nl *BlockchainNodeList) DeepCopyObject() runtime.Object {
	if nl == nil {
		return nil
	}
	out := new(BlockchainNodeList)
	*out = *nl
	if nl.Items != nil {
		out.Items = make([]BlockchainNode, len(nl.Items))
		for i := range nl.Items {
			nl.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}
