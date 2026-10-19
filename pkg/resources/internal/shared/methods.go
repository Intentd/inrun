package shared

import (
	"github.com/intentd/inrun/pkg/resources/core/types"
	corev1 "k8s.io/api/core/v1"
)

// ResolveNamespace — priority: spec.Namespace → owner namespace → "default"
func ResolveNamespace(owner types.Owner, namespace string) string {
	if namespace != "" {
		return namespace
	}
	if owner.GetNamespace() != "" {
		return owner.GetNamespace()
	}
	return "default"
}

// ToPullSecrets converts a slice of string to a []corev1.LocalObjectReference
// Acceptable as Pull secrets
func ToPullSecrets(names []string) []corev1.LocalObjectReference {
	out := make([]corev1.LocalObjectReference, len(names))
	for i, n := range names {
		out[i] = corev1.LocalObjectReference{Name: n}
	}
	return out
}

// PodSelector returns selector, or app.kubernetes.io/name: name when it is
// empty: the label selector a workload uses to find its pods.
func PodSelector(selector map[string]string, name string) map[string]string {
	if len(selector) > 0 {
		return selector
	}
	return map[string]string{"app.kubernetes.io/name": name}
}

// PodLabels returns labels with selector's labels set, so a workload's pod
// template always matches its selector.
func PodLabels(labels, selector map[string]string) map[string]string {
	out := make(map[string]string, len(labels)+len(selector))
	for k, v := range labels {
		out[k] = v
	}
	for k, v := range selector {
		out[k] = v
	}
	return out
}
