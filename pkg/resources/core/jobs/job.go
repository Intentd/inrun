// pkg/resources/jobs/job.go
package jobs

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Spec is the fully resolved Job specification.
type Spec struct {
	// Name — Job name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// Image — container image. Required.
	Image string

	// Command — container entrypoint. Optional.
	Command []string

	// Args — container arguments. Optional.
	Args []string

	// BackoffLimit — number of retries before the Job is marked Failed.
	// Default: 3.
	BackoffLimit int

	// Labels — applied to Job metadata.
	Labels map[string]string

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string

	// Resources — CPU and memory requests/limits. nil means no limits set.
	Resources *types.ResourceRequirements

	// SecurityContext — container-level security settings.
	SecurityContext *types.ContainerSecurityContext

	// PodSecurity — pod-level security settings.
	PodSecurity *types.PodSecurityContext

	// Volumes / VolumeMounts — pod volumes and container mounts.
	Volumes      []types.VolumeSource
	VolumeMounts []types.VolumeMount

	// Detached leaves out the owner reference, so the Job outlives its owner:
	// for cleanup Jobs run while the owner is being deleted.
	Detached bool

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a Job if it does not already exist.
// Sets an owner reference — use for onCreate Jobs only.
func Create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	return create(ctx, kube, owner, spec)
}

// CreateForDelete creates a Job without an owner reference.
// Use for onDelete Jobs: the CR is being deleted and the Job must outlive it.
func CreateForDelete(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	spec.Detached = true
	return create(ctx, kube, owner, spec)
}

func create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("job.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.Clientset().BatchV1().Jobs(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("job.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("job", spec.Name).
			Str("namespace", namespace).
			Msg("job already exists — skipping create")
		return nil
	}

	job := Build(owner, spec)

	_, err = kube.Clientset().BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("job.Create: creating job %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("job", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("job created")

	return nil
}

// Delete deletes the Job if it exists.
func Delete(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	propagation := metav1.DeletePropagationForeground
	err := kube.Clientset().BatchV1().Jobs(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("job", spec.Name).
				Str("namespace", namespace).
				Msg("job already deleted — skipping")
			return nil
		}
		return fmt.Errorf("job.Delete: deleting job %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("job", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("job deleted")

	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// Build returns the Job spec describes, in spec.Namespace or else the
// owner's namespace, owned by owner unless spec.Detached. It reads nothing
// from the cluster.
func Build(owner domain.Object, spec Spec) *batchv1.Job {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	backoffLimit := int32(spec.BackoffLimit)

	container := corev1.Container{
		Name:    spec.Name,
		Image:   spec.Image,
		Command: spec.Command,
		Args:    spec.Args,
	}
	if spec.Resources != nil {
		container.Resources = shared.BuildResourceRequirements(spec.Resources)
	}

	var ownerRefs []metav1.OwnerReference
	if !spec.Detached {
		ownerRefs = shared.ResolveOwnerReferences(owner)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: ownerRefs,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: spec.Labels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: shared.ToPullSecrets(spec.ImagePullSecrets),
					RestartPolicy:    corev1.RestartPolicyOnFailure,
					Containers:       []corev1.Container{container},
				},
			},
		},
	}

	// Security
	shared.ApplySecurityContext(&job.Spec.Template.Spec.Containers[0], &job.Spec.Template.Spec, spec.SecurityContext, spec.PodSecurity)

	// Volumes / VolumeMounts
	if vols := shared.BuildVolumes(spec.Volumes); len(vols) > 0 {
		job.Spec.Template.Spec.Volumes = vols
	}
	if mounts := shared.BuildVolumeMounts(spec.VolumeMounts); len(mounts) > 0 {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = mounts
	}

	return job
}

func validateSpec(spec Spec) error {
	var missing []string
	if spec.Name == "" {
		missing = append(missing, "name")
	}
	if spec.Image == "" {
		missing = append(missing, "image")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}
	return nil
}
