package resources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/intentd/kubewright/core/cronjobs"
	"github.com/intentd/kubewright/core/deployments"
	"github.com/intentd/kubewright/core/jobs"
	"github.com/intentd/kubewright/core/pods"
	"github.com/intentd/kubewright/core/replicasets"
	"github.com/intentd/kubewright/core/statefulsets"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolvePod resolves all template expressions in a PodTemplateSource.
// Returns a new PodTemplateSource with all expressions evaluated — safe to pass
// directly to pods.Resolve().
//
// Defaults applied when fields are empty after resolution:
//
//	Name      → ownerName + "-pod"      (applied later in pods.Resolve)
//	Namespace → ownerNamespace          (applied here so downstream has it)
func ResolvePod(r *template.Resolver, src types.PodTemplateSource) (types.PodTemplateSource, error) {
	resolved := types.PodTemplateSource{}

	var err error

	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("pod.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("pod.resources: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("pod.name: %w", err)
	}
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("pod.image: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("pod.imagePullSecrets: %w", err)
	}
	if resolved.Port, err = r.Resolve(src.Port); err != nil {
		return resolved, fmt.Errorf("pod.port: %w", err)
	}

	// Namespace — default to owner namespace when not declared
	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("pod.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("pod.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("pod.annotations: %w", err)
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("pod.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("pod.volumeMounts: %w", err)
	}

	return resolved, nil
}

// ResolveDeployment resolves all template expressions in a DeploymentTemplateSource.
// Returns a new DeploymentTemplateSource with all expressions evaluated — safe to pass
// directly to deployments.Resolve().
func ResolveDeployment(r *template.Resolver, src types.DeploymentTemplateSource) (types.DeploymentTemplateSource, error) {
	resolved := types.DeploymentTemplateSource{}

	var err error

	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("deployment.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("deployment.resources: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("deployment.name: %w", err)
	}
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("deployment.image: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("deployment.imagePullSecrets: %w", err)
	}
	if resolved.Replicas, err = r.Resolve(src.Replicas); err != nil {
		return resolved, fmt.Errorf("deployment.replicas: %w", err)
	}
	if resolved.Port, err = r.Resolve(src.Port); err != nil {
		return resolved, fmt.Errorf("deployment.port: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("deployment.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("deployment.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("deployment.annotations: %w", err)
	}
	// service account name
	if resolved.ServiceAccountName, err = r.Resolve(src.ServiceAccountName); err != nil {
		return resolved, fmt.Errorf("deployment.serviceAccountName: %w", err)
	}

	// Env resolution
	if len(src.Env) > 0 {
		resolved.Env = make([]types.EnvVar, 0, len(src.Env))
		for _, v := range src.Env {
			ev := types.EnvVar{Name: v.Name}
			if v.Value != "" {
				if ev.Value, err = r.Resolve(v.Value); err != nil {
					return resolved, fmt.Errorf("env[%s].value: %w", v.Name, err)
				}
			}
			if v.ValueFrom != nil {
				ev.ValueFrom = &types.ValueFrom{}
				if v.ValueFrom.SecretKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.SecretKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.secretKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.SecretKeyRef = &types.SecretKeyRef{Name: name, Key: v.ValueFrom.SecretKeyRef.Key}
				}
				if v.ValueFrom.ConfigMapKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.ConfigMapKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.configMapKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.ConfigMapKeyRef = &types.ConfigMapKeyRef{Name: name, Key: v.ValueFrom.ConfigMapKeyRef.Key}
				}
			}
			resolved.Env = append(resolved.Env, ev)
		}
	}

	// EnvFrom resolution
	if src.EnvFrom != nil {
		resolved.EnvFrom = &types.EnvFrom{}
		var everr error
		if resolved.EnvFrom.SecretRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.SecretRef, "envFrom.secretRef"); everr != nil {
			return resolved, everr
		}
		if resolved.EnvFrom.ConfigMapRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.ConfigMapRef, "envFrom.configMapRef"); everr != nil {
			return resolved, everr
		}
	}

	if src.RollingUpdate != nil {
		ru := *src.RollingUpdate
		resolved.RollingUpdate = &ru
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("deployment.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("deployment.volumeMounts: %w", err)
	}

	resolved.Autoscale = src.Autoscale

	return resolved, nil
}

// ResolveReplicaSet resolves all template expressions in a ReplicaSetTemplateSource.
// Returns a new ReplicaSetTemplateSource with all expressions evaluated — safe to pass
// directly to replicasets.Resolve().
func ResolveReplicaSet(r *template.Resolver, src types.ReplicaSetTemplateSource) (types.ReplicaSetTemplateSource, error) {
	resolved := types.ReplicaSetTemplateSource{}

	var err error

	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("replicaset.probes: %w", err)
	}

	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("replicaset.resources: %w", err)
	}

	// Name
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("replicaset.name: %w", err)
	}

	// Image
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("replicaset.image: %w", err)
	}

	// ImagePullSecrets
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("replicaset.imagePullSecrets: %w", err)
	}

	// Replicas
	if resolved.Replicas, err = r.Resolve(src.Replicas); err != nil {
		return resolved, fmt.Errorf("replicaset.replicas: %w", err)
	}

	// Port
	if resolved.Port, err = r.Resolve(src.Port); err != nil {
		return resolved, fmt.Errorf("replicaset.port: %w", err)
	}

	// Namespace (default to CR namespace)
	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("replicaset.namespace: %w", err)
	}

	// service account name
	if resolved.ServiceAccountName, err = r.Resolve(src.ServiceAccountName); err != nil {
		return resolved, fmt.Errorf("replicaet.serviceAccountName: %w", err)
	}

	// Labels
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("replicaset.labels: %w", err)
	}

	// Annotations
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("replicaset.annotations: %w", err)
	}

	// Env
	if len(src.Env) > 0 {
		resolved.Env = make([]types.EnvVar, 0, len(src.Env))
		for _, v := range src.Env {
			ev := types.EnvVar{Name: v.Name}
			if v.Value != "" {
				if ev.Value, err = r.Resolve(v.Value); err != nil {
					return resolved, fmt.Errorf("env[%s].value: %w", v.Name, err)
				}
			}
			if v.ValueFrom != nil {
				ev.ValueFrom = &types.ValueFrom{}
				if v.ValueFrom.SecretKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.SecretKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.secretKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.SecretKeyRef = &types.SecretKeyRef{Name: name, Key: v.ValueFrom.SecretKeyRef.Key}
				}
				if v.ValueFrom.ConfigMapKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.ConfigMapKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.configMapKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.ConfigMapKeyRef = &types.ConfigMapKeyRef{Name: name, Key: v.ValueFrom.ConfigMapKeyRef.Key}
				}
			}
			resolved.Env = append(resolved.Env, ev)
		}
	}

	// EnvFrom
	if src.EnvFrom != nil {
		resolved.EnvFrom = &types.EnvFrom{}
		var everr error
		if resolved.EnvFrom.SecretRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.SecretRef, "envFrom.secretRef"); everr != nil {
			return resolved, everr
		}
		if resolved.EnvFrom.ConfigMapRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.ConfigMapRef, "envFrom.configMapRef"); everr != nil {
			return resolved, everr
		}
	}

	if src.RollingUpdate != nil {
		ru := *src.RollingUpdate
		resolved.RollingUpdate = &ru
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("replicaset.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("replicaset.volumeMounts: %w", err)
	}

	resolved.Autoscale = src.Autoscale

	return resolved, nil
}

// ResolveStatefulSet resolves all template expressions in a StatefulSetTemplateSource.
func ResolveStatefulSet(r *template.Resolver, src types.StatefulSetTemplateSource) (types.StatefulSetTemplateSource, error) {
	resolved := types.StatefulSetTemplateSource{}

	var err error

	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("statefulset.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("statefulset.resources: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("statefulset.name: %w", err)
	}
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("statefulset.image: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("statefulset.imagePullSecrets: %w", err)
	}
	if resolved.Tag, err = r.Resolve(src.Tag); err != nil {
		return resolved, fmt.Errorf("statefulset.tag: %w", err)
	}
	if resolved.Replicas, err = r.Resolve(src.Replicas); err != nil {
		return resolved, fmt.Errorf("statefulset.replicas: %w", err)
	}
	if resolved.Port, err = r.Resolve(src.Port); err != nil {
		return resolved, fmt.Errorf("statefulset.port: %w", err)
	}
	if resolved.ServiceName, err = r.Resolve(src.ServiceName); err != nil {
		return resolved, fmt.Errorf("statefulset.serviceName: %w", err)
	}
	for i, vct := range src.VolumeClaimTemplates {
		rv := types.VolumeClaimTemplateSource{AccessModes: vct.AccessModes}
		if rv.Name, err = r.Resolve(vct.Name); err != nil {
			return resolved, fmt.Errorf("statefulset.volumeClaimTemplates[%d].name: %w", i, err)
		}
		if rv.StorageClass, err = r.Resolve(vct.StorageClass); err != nil {
			return resolved, fmt.Errorf("statefulset.volumeClaimTemplates[%d].storageClass: %w", i, err)
		}
		if rv.StorageSize, err = r.Resolve(vct.StorageSize); err != nil {
			return resolved, fmt.Errorf("statefulset.volumeClaimTemplates[%d].storageSize: %w", i, err)
		}
		if rv.MountPath, err = r.Resolve(vct.MountPath); err != nil {
			return resolved, fmt.Errorf("statefulset.volumeClaimTemplates[%d].mountPath: %w", i, err)
		}
		resolved.VolumeClaimTemplates = append(resolved.VolumeClaimTemplates, rv)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("statefulset.namespace: %w", err)
	}
	// service account name
	if resolved.ServiceAccountName, err = r.Resolve(src.ServiceAccountName); err != nil {
		return resolved, fmt.Errorf("statefulset.serviceAccountName: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("statefulset.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("statefulset.annotations: %w", err)
	}

	// Env resolution
	if len(src.Env) > 0 {
		resolved.Env = make([]types.EnvVar, 0, len(src.Env))
		for _, v := range src.Env {
			ev := types.EnvVar{Name: v.Name}
			if v.Value != "" {
				if ev.Value, err = r.Resolve(v.Value); err != nil {
					return resolved, fmt.Errorf("env[%s].value: %w", v.Name, err)
				}
			}
			if v.ValueFrom != nil {
				ev.ValueFrom = &types.ValueFrom{}
				if v.ValueFrom.SecretKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.SecretKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.secretKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.SecretKeyRef = &types.SecretKeyRef{Name: name, Key: v.ValueFrom.SecretKeyRef.Key}
				}
				if v.ValueFrom.ConfigMapKeyRef != nil {
					name, nerr := r.Resolve(v.ValueFrom.ConfigMapKeyRef.Name)
					if nerr != nil {
						return resolved, fmt.Errorf("env[%s].valueFrom.configMapKeyRef.name: %w", v.Name, nerr)
					}
					ev.ValueFrom.ConfigMapKeyRef = &types.ConfigMapKeyRef{Name: name, Key: v.ValueFrom.ConfigMapKeyRef.Key}
				}
			}
			resolved.Env = append(resolved.Env, ev)
		}
	}

	// EnvFrom resolution
	if src.EnvFrom != nil {
		resolved.EnvFrom = &types.EnvFrom{}
		var everr error
		if resolved.EnvFrom.SecretRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.SecretRef, "envFrom.secretRef"); everr != nil {
			return resolved, everr
		}
		if resolved.EnvFrom.ConfigMapRef, everr = r.ResolveEnvFromRefs(src.EnvFrom.ConfigMapRef, "envFrom.configMapRef"); everr != nil {
			return resolved, everr
		}
	}

	if src.RollingUpdate != nil {
		ru := *src.RollingUpdate
		resolved.RollingUpdate = &ru
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("statefulset.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("statefulset.volumeMounts: %w", err)
	}

	resolved.Autoscale = src.Autoscale

	return resolved, nil
}

// ResolveJob resolves all template expressions in a JobTemplateSource.
func ResolveJob(r *template.Resolver, src types.JobTemplateSource) (types.JobTemplateSource, error) {
	resolved := types.JobTemplateSource{
		BackoffLimit: src.BackoffLimit,
	}

	var err error

	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("job.resources: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("job.name: %w", err)
	}
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("job.image: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("job.imagePullSecrets: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("job.namespace: %w", err)
	}
	// service account name
	if resolved.ServiceAccountName, err = r.Resolve(src.ServiceAccountName); err != nil {
		return resolved, fmt.Errorf("job.serviceAccountName: %w", err)
	}

	// Each command element resolved independently
	for i, c := range src.Command {
		rv, e := r.Resolve(c)
		if e != nil {
			return resolved, fmt.Errorf("job.command[%d]: %w", i, e)
		}
		resolved.Command = append(resolved.Command, rv)
	}
	for i, a := range src.Args {
		rv, e := r.Resolve(a)
		if e != nil {
			return resolved, fmt.Errorf("job.args[%d]: %w", i, e)
		}
		resolved.Args = append(resolved.Args, rv)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("job.labels: %w", err)
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("job.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("job.volumeMounts: %w", err)
	}

	return resolved, nil
}

// ResolveCronJob resolves all template expressions in a CronJobTemplateSource.
func ResolveCronJob(r *template.Resolver, src types.CronJobTemplateSource) (types.CronJobTemplateSource, error) {
	resolved := types.CronJobTemplateSource{}

	var err error

	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("cronjob.resources: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("cronjob.name: %w", err)
	}
	if resolved.Image, err = r.Resolve(src.Image); err != nil {
		return resolved, fmt.Errorf("cronjob.image: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("cronjob.imagePullSecrets: %w", err)
	}
	if resolved.Schedule, err = r.Resolve(src.Schedule); err != nil {
		return resolved, fmt.Errorf("cronjob.schedule: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("cronjob.namespace: %w", err)
	}
	// service account name
	if resolved.ServiceAccountName, err = r.Resolve(src.ServiceAccountName); err != nil {
		return resolved, fmt.Errorf("cronjob.serviceAccountName: %w", err)
	}

	for i, c := range src.Command {
		rv, e := r.Resolve(c)
		if e != nil {
			return resolved, fmt.Errorf("cronjob.command[%d]: %w", i, e)
		}
		resolved.Command = append(resolved.Command, rv)
	}
	for i, a := range src.Args {
		rv, e := r.Resolve(a)
		if e != nil {
			return resolved, fmt.Errorf("cronjob.args[%d]: %w", i, e)
		}
		resolved.Args = append(resolved.Args, rv)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("cronjob.labels: %w", err)
	}

	resolved.SecurityContext = src.SecurityContext
	resolved.PodSecurity = src.PodSecurity

	return resolved, nil
}

// CronJobSpec builds a cronjobs.Spec from a CronJobTemplateSource.
// All template expressions in src must already have been evaluated by
// template.Resolver — Resolve only performs type conversion and defaults.
func CronJobSpec(src types.CronJobTemplateSource, owner domain.Object, reg *types.PresetRegistry) cronjobs.Spec {
	ownerName := owner.GetName()
	spec := cronjobs.Spec{
		Name:            src.Name,
		Namespace:       src.Namespace,
		Schedule:        src.Schedule,
		Image:           src.Image,
		Command:         src.Command,
		Args:            src.Args,
		Labels:          make(map[string]string),
		Resources:       presetResources(src.Resources, reg.All()),
		SecurityContext: presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:     presetPodSecurity(src.PodSecurity, reg.All()),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-cronjob"
	}

	// ── Suspend ───────────────────────────────────────────────────────────
	if src.Suspend != "" {
		spec.Suspend = parseBool(src.Suspend)
	}

	// ── ConcurrencyPolicy ─────────────────────────────────────────────────
	switch strings.ToLower(src.ConcurrencyPolicy) {
	case "forbid":
		spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	case "replace":
		spec.ConcurrencyPolicy = batchv1.ReplaceConcurrent
	default:
		spec.ConcurrencyPolicy = batchv1.AllowConcurrent
	}

	// ── StartingDeadlineSeconds ───────────────────────────────────────────
	if src.StartingDeadlineSeconds != "" {
		if n, err := strconv.ParseInt(src.StartingDeadlineSeconds, 10, 64); err == nil && n > 0 {
			spec.StartingDeadlineSeconds = &n
		}
	}

	// ── SuccessfulJobsHistoryLimit ────────────────────────────────────────
	if src.SuccessfulJobsHistoryLimit != "" {
		if n, err := strconv.ParseInt(src.SuccessfulJobsHistoryLimit, 10, 32); err == nil {
			n32 := int32(n)
			spec.SuccessfulJobsHistoryLimit = &n32
		}
	} else {
		n := int32(3) // Kubernetes default
		spec.SuccessfulJobsHistoryLimit = &n
	}

	// ── FailedJobsHistoryLimit ────────────────────────────────────────────
	if src.FailedJobsHistoryLimit != "" {
		if n, err := strconv.ParseInt(src.FailedJobsHistoryLimit, 10, 32); err == nil {
			n32 := int32(n)
			spec.FailedJobsHistoryLimit = &n32
		}
	} else {
		n := int32(1) // Kubernetes default
		spec.FailedJobsHistoryLimit = &n
	}

	// ── Labels ────────────────────────────────────────────────────────────
	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// DeploymentSpec builds a deployments.Spec from a DeploymentTemplateSource.
// Fields with template expressions must already be evaluated before calling Resolve.
// Use pkg/template.Resolver to evaluate expressions first.
//
// The resolver already evaluated template expressions — here we just merge.
func DeploymentSpec(src types.DeploymentTemplateSource, owner domain.Object, reg *types.PresetRegistry) deployments.Spec {
	ownerName := owner.GetName()
	spec := deployments.Spec{
		Name:            src.Name,
		Image:           src.Image,
		Namespace:       src.Namespace,
		Resources:       presetResources(src.Resources, reg.All()),
		Labels:          make(map[string]string),
		Annotations:     make(map[string]string),
		EnvFrom:         src.EnvFrom,
		Probes:          presetProbes(src.Probes, reg.All()),
		SecurityContext: presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:     presetPodSecurity(src.PodSecurity, reg.All()),
		Command:         src.Command,
		Volumes:         src.Volumes,
		VolumeMounts:    src.VolumeMounts,

		ServiceAccountName: src.ServiceAccountName,
		NodeSelector:       src.NodeSelector,
		ImagePullSecrets:   src.ImagePullSecrets,
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-deployment"
	}

	spec.Replicas = parseReplicas(src.Replicas)

	// Port — prefer dynamic resolved string, fall back to static int
	if src.Port != "" {
		if p, err := strconv.ParseInt(src.Port, 10, 32); err == nil {
			spec.Port = int32(p)
		}
	}

	spec.Protocol = parseProtocol(src.Protocol)

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	spec.Env = []types.EnvVar(src.Env)

	spec.RollingUpdate = presetRollingUpdate(src.RollingUpdate, reg.All())

	// Inrun system labels — always added

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	return spec
}

// JobSpec builds a jobs.Spec from a JobTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func JobSpec(src types.JobTemplateSource, backoffLimit int, owner domain.Object, reg *types.PresetRegistry) jobs.Spec {
	ownerName := owner.GetName()
	spec := jobs.Spec{
		Name:            src.Name,
		Namespace:       src.Namespace,
		Image:           src.Image,
		Command:         src.Command,
		Args:            src.Args,
		BackoffLimit:    backoffLimit,
		Labels:          make(map[string]string),
		Resources:       presetResources(src.Resources, reg.All()),
		SecurityContext: presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:     presetPodSecurity(src.PodSecurity, reg.All()),
		Volumes:         src.Volumes,
		VolumeMounts:    src.VolumeMounts,
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-job"
	}
	if spec.BackoffLimit == 0 {
		spec.BackoffLimit = 3
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	// System labels

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// PodSpec builds a pods.Spec from a PodTemplateSource.
//
// Fields are flat on the source struct. Template expressions
// must already be evaluated by template.Resolver before calling Resolve.
// This function only reads already-resolved string values and assembles the spec.
//
// Inrun system labels (managed-by, inrun-owner) are always added
// and cannot be overridden by the user.
func PodSpec(src types.PodTemplateSource, owner domain.Object, reg *types.PresetRegistry) pods.Spec {
	ownerName := owner.GetName()
	spec := pods.Spec{
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}

	spec.Name = src.Name
	if spec.Name == "" {
		spec.Name = ownerName + "-pod"
	}

	spec.Image = src.Image
	spec.Namespace = src.Namespace
	spec.Resources = src.Resources
	spec.Probes = presetProbes(src.Probes, reg.All())
	spec.SecurityContext = presetContainerSecurity(src.SecurityContext, reg.All())
	spec.PodSecurity = presetPodSecurity(src.PodSecurity, reg.All())
	spec.Volumes = src.Volumes
	spec.VolumeMounts = src.VolumeMounts

	if src.Port != "" {
		spec.Port = parsePort(src.Port)
	}
	spec.Protocol = parseProtocol(src.Protocol)

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	// System labels — always present

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// ReplicaSetSpec builds a replicasets.Spec from a ReplicaSetTemplateSource.
func ReplicaSetSpec(src types.ReplicaSetTemplateSource, owner domain.Object, reg *types.PresetRegistry) replicasets.Spec {
	ownerName := owner.GetName()
	spec := replicasets.Spec{
		Name:            src.Name,
		Image:           src.Image,
		Namespace:       src.Namespace,
		Resources:       presetResources(src.Resources, reg.All()),
		Labels:          make(map[string]string),
		Annotations:     make(map[string]string),
		EnvFrom:         src.EnvFrom,
		Probes:          presetProbes(src.Probes, reg.All()),
		SecurityContext: presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:     presetPodSecurity(src.PodSecurity, reg.All()),
		Volumes:         src.Volumes,
		VolumeMounts:    src.VolumeMounts,

		ServiceAccountName: src.ServiceAccountName,
		NodeSelector:       src.NodeSelector,
		ImagePullSecrets:   src.ImagePullSecrets,
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-replicaset"
	}

	spec.Replicas = parseReplicas(src.Replicas)

	if src.Port != "" {
		if p, err := strconv.ParseInt(src.Port, 10, 32); err == nil {
			spec.Port = int32(p)
		}
	}
	spec.Protocol = parseProtocol(src.Protocol)

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	spec.Env = []types.EnvVar(src.Env)

	spec.RollingUpdate = presetRollingUpdate(src.RollingUpdate, reg.All())

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	return spec
}

// StatefulSetSpec builds a statefulsets.Spec from a StatefulSetTemplateSource.
func StatefulSetSpec(src types.StatefulSetTemplateSource, owner domain.Object, reg *types.PresetRegistry) statefulsets.Spec {
	ownerName := owner.GetName()
	spec := statefulsets.Spec{
		Name:            src.Name,
		Namespace:       src.Namespace,
		Image:           src.Image,
		ServiceName:     src.ServiceName,
		Replicas:        parseReplicas(src.Replicas),
		Labels:          make(map[string]string),
		Annotations:     make(map[string]string),
		Env:             src.Env,
		EnvFrom:         src.EnvFrom,
		Resources:       presetResources(src.Resources, reg.All()),
		Probes:          presetProbes(src.Probes, reg.All()),
		SecurityContext: presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:     presetPodSecurity(src.PodSecurity, reg.All()),
		Volumes:         src.Volumes,
		VolumeMounts:    src.VolumeMounts,

		ServiceAccountName: src.ServiceAccountName,
		NodeSelector:       src.NodeSelector,
		ImagePullSecrets:   src.ImagePullSecrets,
	}

	for _, vct := range src.VolumeClaimTemplates {
		resolved := statefulsets.VolumeClaimTemplate{
			Name:         vct.Name,
			StorageClass: vct.StorageClass,
			StorageSize:  vct.StorageSize,
			MountPath:    vct.MountPath,
			AccessModes:  vct.AccessModes,
		}
		if resolved.Name == "" {
			resolved.Name = "data"
		}
		if resolved.MountPath == "" {
			resolved.MountPath = "/data"
		}
		spec.VolumeClaimTemplates = append(spec.VolumeClaimTemplates, resolved)
	}

	if spec.Name == "" {
		spec.Name = ownerName
	}
	if spec.ServiceName == "" {
		spec.ServiceName = spec.Name
	}

	if src.Tag != "" {
		spec.Image = src.Image + ":" + src.Tag
	}
	if p, err := strconv.ParseInt(src.Port, 10, 32); err == nil {
		spec.Port = int32(p)
	}
	spec.Protocol = parseProtocol(src.Protocol)

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	spec.RollingUpdate = presetRollingUpdate(src.RollingUpdate, reg.All())

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	return spec
}
