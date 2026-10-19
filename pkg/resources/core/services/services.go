// pkg/resources/services/service.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
)

// Create creates a Service owned by the CR if it does not already exist.
// Idempotent — if the Service exists, does nothing and returns nil.
// Owner reference is set so the Service is garbage collected when the CR is deleted.
func Create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("service.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.Clientset().CoreV1().Services(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("service.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("service", spec.Name).
			Str("namespace", namespace).
			Msg("service already exists — skipping create")
		return nil
	}

	svc := Build(owner, spec)

	_, err = kube.Clientset().CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("service.Create: creating service %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("service", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("service created")

	return nil
}

// Apply creates or updates a Service using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("service.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	svc := Build(owner, spec)
	svc.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}

	body, err := json.Marshal(svc)
	if err != nil {
		return fmt.Errorf("service.Apply: marshal: %w", err)
	}

	if _, err = kube.Clientset().CoreV1().Services(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		metav1.PatchOptions{FieldManager: config.FieldManagerRuntime, Force: shared.ResolveForceConflict(kube, spec.ForceConflict)},
	); err != nil {
		return fmt.Errorf("service.Apply: %w", err)
	}

	logger.Debug().
		Str("service", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("service applied")

	return nil
}

// Delete deletes the Service if it exists.
func Delete(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.Clientset().CoreV1().Services(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("service", spec.Name).
				Str("namespace", namespace).
				Msg("service already deleted — skipping")
			return nil
		}
		return fmt.Errorf("service.Delete: deleting service %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("service", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("service deleted")

	return nil
}

// DeleteIfOwned deletes the Service if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubeclient.Interface,
	owner domain.Object, name, namespace string) error {

	existing, err := kube.Clientset().CoreV1().Services(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// Only delete if we own it
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.Clientset().CoreV1().Services(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Service spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner domain.Object, spec Spec) *corev1.Service {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	svcType := corev1.ServiceTypeClusterIP
	switch spec.Type {
	case "NodePort":
		svcType = corev1.ServiceTypeNodePort
	case "LoadBalancer":
		svcType = corev1.ServiceTypeLoadBalancer
	}

	if spec.Headless {
		svcType = corev1.ClusterIPNone
	}

	proto := corev1.ProtocolTCP
	switch strings.ToUpper(spec.Protocol) {
	case "UDP":
		proto = corev1.ProtocolUDP
	case "SCTP":
		proto = corev1.ProtocolSCTP
	case "", "TCP":
		proto = corev1.ProtocolTCP
	}

	// Selector matches pods created by deployments with the same owner
	spec.Selector["inrun-owner"] = owner.GetName()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: spec.Selector,
			Ports: []corev1.ServicePort{
				{
					Port:       spec.Port,
					TargetPort: intstr.FromInt(int(spec.TargetPort)),
					Protocol:   proto,
				},
			},
		},
	}
}

func validateSpec(spec Spec) error {
	var missing []string
	if spec.Name == "" {
		missing = append(missing, "name")
	}
	if spec.Port == 0 {
		missing = append(missing, "port")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}
	return nil
}
