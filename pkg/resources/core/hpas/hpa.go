// pkg/resources/hpas/hpa.go
package hpas

import (
	"context"
	"encoding/json"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Create creates an HPA owned by the CR if it does not already exist.
// Idempotent — if the HPA exists, does nothing and returns nil.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("hpa.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("hpa.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	hpa := Build(owner, spec)

	_, err = kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(ctx, hpa, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("hpa.Create: creating hpa %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates an HPA using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("hpa.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	hpa := Build(owner, spec)
	hpa.TypeMeta = metav1.TypeMeta{APIVersion: "autoscaling/v2", Kind: "HorizontalPodAutoscaler"}

	body, err := json.Marshal(hpa)
	if err != nil {
		return "", fmt.Errorf("hpa.Apply: marshal: %w", err)
	}

	if _, err = kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("hpa.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the HPA if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("hpa.Delete: deleting hpa %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the HPA if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", err
	}
	if !owned(existing) {
		return types.NotOwned, nil
	}
	if err := kube.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the HorizontalPodAutoscaler spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *autoscalingv2.HorizontalPodAutoscaler {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	minR := spec.MinReplicas

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: spec.ScaleTargetRef.APIVersion,
				Kind:       spec.ScaleTargetRef.Kind,
				Name:       spec.ScaleTargetRef.Name,
			},
			MinReplicas: &minR,
			MaxReplicas: spec.MaxReplicas,
		},
	}

	if spec.TargetCPUUtilizationPercentage > 0 {
		cpuTarget := spec.TargetCPUUtilizationPercentage
		hpa.Spec.Metrics = []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &cpuTarget,
					},
				},
			},
		}
	}

	if spec.Behavior != nil {
		hpa.Spec.Behavior = buildK8sBehavior(spec.Behavior)
	}

	return hpa
}

func buildK8sBehavior(b *types.HPABehavior) *autoscalingv2.HorizontalPodAutoscalerBehavior {
	k8s := &autoscalingv2.HorizontalPodAutoscalerBehavior{}

	if b.ScaleUp != nil {
		k8s.ScaleUp = buildK8sScalingRules(b.ScaleUp)
	}
	if b.ScaleDown != nil {
		k8s.ScaleDown = buildK8sScalingRules(b.ScaleDown)
	}
	return k8s
}

func buildK8sScalingRules(r *types.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	k8s := &autoscalingv2.HPAScalingRules{}

	sw := r.StabilizationWindowSeconds
	k8s.StabilizationWindowSeconds = &sw

	if r.SelectPolicy != "" {
		sp := autoscalingv2.ScalingPolicySelect(r.SelectPolicy)
		k8s.SelectPolicy = &sp
	}

	for _, p := range r.Policies {
		k8s.Policies = append(k8s.Policies, autoscalingv2.HPAScalingPolicy{
			Type:          autoscalingv2.HPAScalingPolicyType(p.Type),
			Value:         p.Value,
			PeriodSeconds: p.PeriodSeconds,
		})
	}
	return k8s
}

func behaviorEqual(a, b *autoscalingv2.HorizontalPodAutoscalerBehavior) bool {
	if a == nil && b == nil {
		return true
	}
	if b == nil {
		// No desired behavior — nothing to enforce, not a drift.
		return true
	}
	if a == nil {
		return false
	}
	// Only compare a side if we have an explicit desired spec for it.
	// Kubernetes injects defaults for the unset side; ignore those.
	if b.ScaleUp != nil && !scalingRulesEqual(a.ScaleUp, b.ScaleUp) {
		return false
	}
	if b.ScaleDown != nil && !scalingRulesEqual(a.ScaleDown, b.ScaleDown) {
		return false
	}
	return true
}

func scalingRulesEqual(a, b *autoscalingv2.HPAScalingRules) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	swA := int32(0)
	if a.StabilizationWindowSeconds != nil {
		swA = *a.StabilizationWindowSeconds
	}
	swB := int32(0)
	if b.StabilizationWindowSeconds != nil {
		swB = *b.StabilizationWindowSeconds
	}
	if swA != swB {
		return false
	}
	// Only compare SelectPolicy when we explicitly set one — Kubernetes injects
	// a default ("Max") when the field is omitted, which would otherwise loop.
	if b.SelectPolicy != nil {
		spA := autoscalingv2.ScalingPolicySelect("")
		if a.SelectPolicy != nil {
			spA = *a.SelectPolicy
		}
		if spA != *b.SelectPolicy {
			return false
		}
	}
	// Only compare Policies when we declared any — same default-injection risk.
	if len(b.Policies) > 0 {
		if len(a.Policies) != len(b.Policies) {
			return false
		}
		for i := range b.Policies {
			if a.Policies[i] != b.Policies[i] {
				return false
			}
		}
	}
	return true
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	return nil
}
