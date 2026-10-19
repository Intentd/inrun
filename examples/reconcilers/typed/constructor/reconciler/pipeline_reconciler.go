//go:build ignore

package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/template/resources"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/intentd/inrun-constructor-demo/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/jobs"
	"github.com/intentd/inrun/pkg/types"
)

const (
	finalizerName = "inrun.dev/pipeline-cleanup"
	backoffLimit  = 5
)

// PipelineReconciler implements domain.Reconciler directly; the generic
// reconciler is not used. Inrun still runs the informer, queue, workers,
// panic recovery, metrics and health. This file owns everything else: cache
// reads, the finalizer, events, status and the phase machine
// (Pending → Running → Succeeded | Failed).
type PipelineReconciler struct {
	kube kubeclient.Interface
}

// NewPipelineReconciler is the constructor function registered in the Blueprint.
func NewPipelineReconciler(kube kubeclient.Interface) domain.Reconciler {
	return &PipelineReconciler{kube: kube}
}

// Reconcile is called by Inrun's worker pool for every queued Pipeline key.
// It is wrapped in safeReconcile — panics are caught and returned as errors.
func (r *PipelineReconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	if req.Prepared == nil {
		return domain.Result{}, nil
	}

	pipeline, err := domain.ToTyped[apiv1.Pipeline](req.Prepared)
	if err != nil {
		return domain.Result{}, fmt.Errorf("toTyped: %w", err)
	}

	// ── Deletion handling ──────────────────────────────────────────────────
	if pipeline.DeletionTimestamp != nil {
		return domain.Result{}, r.handleDeletion(ctx, pipeline)
	}

	// ── Finalizer ─────────────────────────────────────────────────────────
	if !containsFinalizer(pipeline, finalizerName) {
		pipeline.Finalizers = append(pipeline.Finalizers, finalizerName)
		if err := r.kube.PatchFinalizers(ctx, pipeline, pipeline.Finalizers, metav1.PatchOptions{}); err != nil {
			return domain.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// ── State machine ─────────────────────────────────────────────────────
	switch pipeline.Status.Phase {
	case "", apiv1.PipelinePhasePending:
		return domain.Result{}, r.handlePending(ctx, pipeline)
	case apiv1.PipelinePhaseRunning:
		return domain.Result{}, r.handleRunning(ctx, pipeline)
	case apiv1.PipelinePhaseSucceeded, apiv1.PipelinePhaseFailed:
		// Terminal state — nothing to reconcile
		return domain.Result{}, nil
	default:
		return domain.Result{}, fmt.Errorf("unknown phase %q for pipeline %s/%s",
			pipeline.Status.Phase, pipeline.Namespace, pipeline.Name)
	}
}

// handlePending transitions the Pipeline from Pending to Running.
// Creates the first step Job and updates status.
func (r *PipelineReconciler) handlePending(ctx context.Context, p *apiv1.Pipeline) error {
	if len(p.Spec.Steps) == 0 {
		return r.setPhase(ctx, p, apiv1.PipelinePhaseSucceeded, "no steps defined")
	}

	firstStep := p.Spec.Steps[0]

	// Create the Job for the first step via pkg/resources
	jobSpec := resources.JobSpec(
		types.JobTemplateSource{
			Name:      fmt.Sprintf("%s-%s", p.Name, firstStep.Name),
			Namespace: p.Namespace,
			Image:     p.Spec.Image,
			Command:   firstStep.Command,
		},
		backoffLimit,
		p.Name,
		nil,
	)
	if _, err := jobs.Create(ctx, r.kube, p, jobSpec); err != nil {
		return fmt.Errorf("creating step job %q: %w", firstStep.Name, err)
	}

	now := metav1.NewTime(time.Now())
	p.Status.Phase = apiv1.PipelinePhaseRunning
	p.Status.CurrentStep = firstStep.Name
	p.Status.StartTime = &now

	r.kube.GetEventRecorder().Eventf(p, corev1.EventTypeNormal, "PipelineStarted",
		"Pipeline %s/%s started — step: %s", p.Namespace, p.Name, firstStep.Name)

	return r.patchStatus(ctx, p)
}

// handleRunning checks whether the current step Job has completed and
// advances to the next step or transitions to Succeeded/Failed.
func (r *PipelineReconciler) handleRunning(ctx context.Context, p *apiv1.Pipeline) error {
	currentJobName := fmt.Sprintf("%s-%s", p.Name, p.Status.CurrentStep)

	// Read the current step Job
	job, err := r.getJob(ctx, p.Namespace, currentJobName)
	if err != nil {
		return fmt.Errorf("getting job %q: %w", currentJobName, err)
	}
	if job == nil {
		// Job not found — may have been deleted externally, recreate
		return r.handlePending(ctx, p)
	}

	// Check Job outcome
	if isJobFailed(job) {
		r.kube.GetEventRecorder().Eventf(p, corev1.EventTypeWarning, "StepFailed",
			"Pipeline step %q failed", p.Status.CurrentStep)
		return r.setPhase(ctx, p, apiv1.PipelinePhaseFailed,
			fmt.Sprintf("step %q failed", p.Status.CurrentStep))
	}

	if !isJobComplete(job) {
		// Still running — requeue implicitly via resync
		return nil
	}

	// Step succeeded — advance to next step
	return r.advanceStep(ctx, p)
}

// advanceStep creates the next step's Job or transitions to Succeeded.
func (r *PipelineReconciler) advanceStep(ctx context.Context, p *apiv1.Pipeline) error {
	// Find current step index
	currentIdx := -1
	for i, step := range p.Spec.Steps {
		if step.Name == p.Status.CurrentStep {
			currentIdx = i
			break
		}
	}

	nextIdx := currentIdx + 1
	if nextIdx >= len(p.Spec.Steps) {
		// All steps complete
		now := metav1.NewTime(time.Now())
		p.Status.CompletionTime = &now
		r.kube.GetEventRecorder().Eventf(p, corev1.EventTypeNormal, "PipelineSucceeded",
			"Pipeline %s/%s completed all %d steps", p.Namespace, p.Name, len(p.Spec.Steps))
		return r.setPhase(ctx, p, apiv1.PipelinePhaseSucceeded, "all steps completed")
	}

	// Create next step Job
	nextStep := p.Spec.Steps[nextIdx]
	jobSpec := resources.JobSpec(
		types.JobTemplateSource{
			Name:      fmt.Sprintf("%s-%s", p.Name, nextStep.Name),
			Namespace: p.Namespace,
			Image:     p.Spec.Image,
			Command:   nextStep.Command,
		},
		backoffLimit,
		p.Name,
		nil,
	)
	if _, err := jobs.Create(ctx, r.kube, p, jobSpec); err != nil {
		return fmt.Errorf("creating step job %q: %w", nextStep.Name, err)
	}

	p.Status.CurrentStep = nextStep.Name
	r.kube.GetEventRecorder().Eventf(p, corev1.EventTypeNormal, "StepStarted",
		"Pipeline advancing to step %q", nextStep.Name)

	return r.patchStatus(ctx, p)
}

// handleDeletion removes the finalizer. Owner references clean up Jobs.
func (r *PipelineReconciler) handleDeletion(ctx context.Context, p *apiv1.Pipeline) error {
	newFinalizers := make([]string, 0, len(p.Finalizers))
	for _, f := range p.Finalizers {
		if f != finalizerName {
			newFinalizers = append(newFinalizers, f)
		}
	}
	return r.kube.PatchFinalizers(ctx, p, newFinalizers, metav1.PatchOptions{})
}

// ── Helpers ───────────────────────────────────────────────────────────────

func (r *PipelineReconciler) setPhase(ctx context.Context, p *apiv1.Pipeline, phase apiv1.PipelinePhase, msg string) error {
	p.Status.Phase = phase
	p.Status.Message = msg
	return r.patchStatus(ctx, p)
}

func (r *PipelineReconciler) patchStatus(ctx context.Context, p *apiv1.Pipeline) error {
	patch := map[string]interface{}{
		"phase":       string(p.Status.Phase),
		"currentStep": p.Status.CurrentStep,
		"message":     p.Status.Message,
	}
	if p.Status.StartTime != nil {
		patch["startTime"] = p.Status.StartTime.Format(time.RFC3339)
	}
	if p.Status.CompletionTime != nil {
		patch["completionTime"] = p.Status.CompletionTime.Format(time.RFC3339)
	}
	return r.kube.PatchStatus(ctx, p, patch, metav1.PatchOptions{})
}

func (r *PipelineReconciler) getJob(ctx context.Context, namespace, name string) (*batchv1.Job, error) {
	obj, err := r.kube.Clientset().BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func isJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func containsFinalizer(p *apiv1.Pipeline, f string) bool {
	for _, fin := range p.Finalizers {
		if fin == f {
			return true
		}
	}
	return false
}

func isNotFound(err error) bool {
	return err != nil && (len(err.Error()) > 0) &&
		(err.Error() == "not found" ||
			contains(err.Error(), "not found"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		len(s) > 0 && len(sub) > 0 &&
			(s[:len(sub)] == sub || contains(s[1:], sub)))
}
