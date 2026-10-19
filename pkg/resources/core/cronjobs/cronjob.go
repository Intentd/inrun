// pkg/resources/cronjobs/cronjob.go
package cronjobs

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Spec is the fully resolved CronJob specification.
// All template expressions have been evaluated before this struct is populated.
type Spec struct {
	// Name — CronJob name. Required.
	Name string

	// Namespace — target namespace.
	Namespace string

	// Schedule — standard cron expression. Required.
	// e.g. "*/5 * * * *", "0 2 * * 1-5"
	Schedule string

	// Image — container image. Required.
	Image string

	// Command — container entrypoint override.
	Command []string

	// Args — container arguments.
	Args []string

	// Suspend — when true, all subsequent executions are suspended.
	// The currently running job is not affected.
	Suspend bool

	// ConcurrencyPolicy — how to treat concurrent executions.
	// One of: Allow (default), Forbid, Replace.
	ConcurrencyPolicy batchv1.ConcurrencyPolicy

	// StartingDeadlineSeconds — deadline for starting the job if it
	// misses its scheduled time. nil means no deadline.
	StartingDeadlineSeconds *int64

	// SuccessfulJobsHistoryLimit — number of successful finished jobs to keep.
	// nil means the Kubernetes default (3).
	SuccessfulJobsHistoryLimit *int32

	// FailedJobsHistoryLimit — number of failed finished jobs to keep.
	// nil means the Kubernetes default (1).
	FailedJobsHistoryLimit *int32

	// Labels — applied to CronJob and pod metadata.
	Labels map[string]string

	// Resources — CPU and memory requests/limits. nil means no limits set.
	Resources *types.ResourceRequirements

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string

	// SecurityContext — container-level security settings.
	SecurityContext *types.ContainerSecurityContext

	// PodSecurity — pod-level security settings.
	PodSecurity *types.PodSecurityContext

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a CronJob if it does not already exist.
// Idempotent — skips creation if the CronJob already exists.
// Sets owner reference so the CronJob is garbage collected when the CR is deleted.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("cronjob.Create: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.BatchV1().CronJobs(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("cronjob.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("cronjob", spec.Name).
			Str("namespace", namespace).
			Msg("cronjob already exists — skipping create")
		return nil
	}

	cj := Build(owner, spec)

	_, err = kube.BatchV1().CronJobs(namespace).Create(ctx, cj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("cronjob.Create: creating %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("cronjob", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("cronjob created")

	return nil
}

// Apply creates or updates a CronJob using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("cronjob.Apply: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	cj := Build(owner, spec)
	cj.TypeMeta = metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"}

	body, err := json.Marshal(cj)
	if err != nil {
		return fmt.Errorf("cronjob.Apply: marshal: %w", err)
	}

	if _, err = kube.BatchV1().CronJobs(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return fmt.Errorf("cronjob.Apply: %w", err)
	}

	logger.Debug().
		Str("cronjob", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("cronjob applied")

	return nil
}

// Delete deletes the CronJob if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.BatchV1().CronJobs(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("cronjob", spec.Name).
				Str("namespace", namespace).
				Msg("cronjob already deleted — skipping")
			return nil
		}
		return fmt.Errorf("cronjob.Delete: deleting %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("cronjob", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("cronjob deleted")

	return nil
}

// DeleteIfOwned deletes the CronJob only if it is labelled as owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface,
	owner types.Owner, name, namespace string) error {

	existing, err := kube.BatchV1().CronJobs(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("cronjob.DeleteIfOwned: getting %q: %w", name, err)
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.BatchV1().CronJobs(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the CronJob spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *batchv1.CronJob {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   spec.Schedule,
			Suspend:                    ptr.To(spec.Suspend),
			ConcurrencyPolicy:          spec.ConcurrencyPolicy,
			StartingDeadlineSeconds:    spec.StartingDeadlineSeconds,
			SuccessfulJobsHistoryLimit: spec.SuccessfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     spec.FailedJobsHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: spec.Labels,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: spec.Labels,
						},
						Spec: corev1.PodSpec{
							ImagePullSecrets: shared.ToPullSecrets(spec.ImagePullSecrets),
							RestartPolicy:    corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								buildContainer(spec),
							},
						},
					},
				},
			},
		},
	}

	// Security
	shared.ApplySecurityContext(
		&cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0],
		&cj.Spec.JobTemplate.Spec.Template.Spec,
		spec.SecurityContext,
		spec.PodSecurity,
	)

	return cj
}

func buildContainer(spec Spec) corev1.Container {
	c := corev1.Container{
		Name:    spec.Name,
		Image:   spec.Image,
		Command: spec.Command,
		Args:    spec.Args,
	}
	if spec.Resources != nil {
		c.Resources = shared.BuildResourceRequirements(spec.Resources)
	}
	return c
}

func validateSpec(spec Spec) error {
	var missing []string
	if spec.Name == "" {
		missing = append(missing, "name")
	}
	if spec.Image == "" {
		missing = append(missing, "image")
	}
	if spec.Schedule == "" {
		missing = append(missing, "schedule")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}
	return nil
}
