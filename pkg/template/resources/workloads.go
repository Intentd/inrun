package resources

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/kubewright/core/cronjobs"
	"github.com/intentd/kubewright/core/deployments"
	"github.com/intentd/kubewright/core/jobs"
	"github.com/intentd/kubewright/core/pods"
	"github.com/intentd/kubewright/core/replicasets"
	"github.com/intentd/kubewright/core/statefulsets"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// The resolvers below start from a copy of the entry, so a field that holds no
// template expression reaches the spec builder as declared, then resolve the
// fields that may. The container fields go through the pod helpers (pod.go),
// shared by all six kinds.

// resolveFields resolves each named string field in place, stopping at the
// first error, which it prefixes with kind and field name.
func resolveFields(r *template.Resolver, kind string, fields map[string]*string) error {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dst := fields[name]
		v, err := r.Resolve(*dst)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", kind, name, err)
		}
		*dst = v
	}
	return nil
}

// ResolvePod resolves all template expressions in a PodTemplateSource.
func ResolvePod(r *template.Resolver, src types.PodTemplateSource) (types.PodTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "pod", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image, "port": &resolved.Port,
		"protocol": &resolved.Protocol, "serviceAccountName": &resolved.ServiceAccountName,
		"restartPolicy": &resolved.RestartPolicy,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("pod.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("pod.imagePullSecrets: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("pod.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("pod.annotations: %w", err)
	}
	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("pod.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("pod.resources: %w", err)
	}
	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("pod.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("pod.volumeMounts: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "pod.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "pod.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// ResolveDeployment resolves all template expressions in a DeploymentTemplateSource.
func ResolveDeployment(r *template.Resolver, src types.DeploymentTemplateSource) (types.DeploymentTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "deployment", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image, "replicas": &resolved.Replicas,
		"port": &resolved.Port, "protocol": &resolved.Protocol,
		"serviceAccountName": &resolved.ServiceAccountName,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("deployment.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("deployment.imagePullSecrets: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("deployment.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("deployment.annotations: %w", err)
	}
	if resolved.Command, err = r.ResolveArgs(src.Command, "deployment.command"); err != nil {
		return resolved, err
	}
	if resolved.Env, err = resolveEnv(r, src.Env); err != nil {
		return resolved, err
	}
	if resolved.EnvFrom, err = resolveEnvFrom(r, src.EnvFrom); err != nil {
		return resolved, err
	}
	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("deployment.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("deployment.resources: %w", err)
	}
	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("deployment.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("deployment.volumeMounts: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "deployment.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "deployment.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// ResolveReplicaSet resolves all template expressions in a ReplicaSetTemplateSource.
func ResolveReplicaSet(r *template.Resolver, src types.ReplicaSetTemplateSource) (types.ReplicaSetTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "replicaset", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image, "replicas": &resolved.Replicas,
		"port": &resolved.Port, "protocol": &resolved.Protocol,
		"serviceAccountName": &resolved.ServiceAccountName,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("replicaset.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("replicaset.imagePullSecrets: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("replicaset.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("replicaset.annotations: %w", err)
	}
	if resolved.Env, err = resolveEnv(r, src.Env); err != nil {
		return resolved, err
	}
	if resolved.EnvFrom, err = resolveEnvFrom(r, src.EnvFrom); err != nil {
		return resolved, err
	}
	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("replicaset.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("replicaset.resources: %w", err)
	}
	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("replicaset.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("replicaset.volumeMounts: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "replicaset.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "replicaset.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// ResolveStatefulSet resolves all template expressions in a StatefulSetTemplateSource.
func ResolveStatefulSet(r *template.Resolver, src types.StatefulSetTemplateSource) (types.StatefulSetTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "statefulset", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image,
		"replicas": &resolved.Replicas, "port": &resolved.Port, "protocol": &resolved.Protocol,
		"serviceName": &resolved.ServiceName, "serviceAccountName": &resolved.ServiceAccountName,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("statefulset.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("statefulset.imagePullSecrets: %w", err)
	}
	resolved.VolumeClaimTemplates = nil
	for i, vct := range src.VolumeClaimTemplates {
		rv := vct
		if err = resolveFields(r, fmt.Sprintf("statefulset.volumeClaimTemplates[%d]", i), map[string]*string{
			"name": &rv.Name, "storageClass": &rv.StorageClass, "storageSize": &rv.StorageSize, "mountPath": &rv.MountPath,
		}); err != nil {
			return resolved, err
		}
		resolved.VolumeClaimTemplates = append(resolved.VolumeClaimTemplates, rv)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("statefulset.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("statefulset.annotations: %w", err)
	}
	if resolved.Env, err = resolveEnv(r, src.Env); err != nil {
		return resolved, err
	}
	if resolved.EnvFrom, err = resolveEnvFrom(r, src.EnvFrom); err != nil {
		return resolved, err
	}
	if resolved.Probes, err = resolveProbes(r, src.Probes); err != nil {
		return resolved, fmt.Errorf("statefulset.probes: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("statefulset.resources: %w", err)
	}
	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("statefulset.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("statefulset.volumeMounts: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "statefulset.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "statefulset.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// ResolveJob resolves all template expressions in a JobTemplateSource.
func ResolveJob(r *template.Resolver, src types.JobTemplateSource) (types.JobTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "job", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image,
		"serviceAccountName": &resolved.ServiceAccountName, "restartPolicy": &resolved.RestartPolicy,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("job.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("job.imagePullSecrets: %w", err)
	}
	if resolved.Command, err = r.ResolveArgs(src.Command, "job.command"); err != nil {
		return resolved, err
	}
	if resolved.Args, err = r.ResolveArgs(src.Args, "job.args"); err != nil {
		return resolved, err
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("job.labels: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("job.resources: %w", err)
	}
	if resolved.Volumes, err = resolveVolumes(r, src.Volumes); err != nil {
		return resolved, fmt.Errorf("job.volumes: %w", err)
	}
	if resolved.VolumeMounts, err = resolveVolumeMounts(r, src.VolumeMounts); err != nil {
		return resolved, fmt.Errorf("job.volumeMounts: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "job.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "job.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// ResolveCronJob resolves all template expressions in a CronJobTemplateSource.
func ResolveCronJob(r *template.Resolver, src types.CronJobTemplateSource) (types.CronJobTemplateSource, error) {
	resolved := src
	var err error
	if err = resolveFields(r, "cronjob", map[string]*string{
		"name": &resolved.Name, "image": &resolved.Image, "schedule": &resolved.Schedule,
		"suspend": &resolved.Suspend, "concurrencyPolicy": &resolved.ConcurrencyPolicy,
		"startingDeadlineSeconds":    &resolved.StartingDeadlineSeconds,
		"successfulJobsHistoryLimit": &resolved.SuccessfulJobsHistoryLimit,
		"failedJobsHistoryLimit":     &resolved.FailedJobsHistoryLimit,
		"serviceAccountName":         &resolved.ServiceAccountName,
		"restartPolicy":              &resolved.RestartPolicy,
	}); err != nil {
		return resolved, err
	}
	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
		return resolved, fmt.Errorf("cronjob.namespace: %w", err)
	}
	if resolved.ImagePullSecrets, err = r.ResolveStringSlice(src.ImagePullSecrets); err != nil {
		return resolved, fmt.Errorf("cronjob.imagePullSecrets: %w", err)
	}
	if resolved.Command, err = r.ResolveArgs(src.Command, "cronjob.command"); err != nil {
		return resolved, err
	}
	if resolved.Args, err = r.ResolveArgs(src.Args, "cronjob.args"); err != nil {
		return resolved, err
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("cronjob.labels: %w", err)
	}
	if resolved.Resources, err = resolveResourceRequirements(r, src.Resources); err != nil {
		return resolved, fmt.Errorf("cronjob.resources: %w", err)
	}
	if resolved.Containers, err = resolveContainers(r, src.Containers, "cronjob.containers"); err != nil {
		return resolved, err
	}
	if resolved.InitContainers, err = resolveContainers(r, src.InitContainers, "cronjob.initContainers"); err != nil {
		return resolved, err
	}
	return resolved, nil
}

// CronJobSpec builds a cronjobs.Spec from a CronJobTemplateSource.
// All template expressions in src must already have been evaluated by
// template.Resolver — Resolve only performs type conversion and defaults.
func CronJobSpec(src types.CronJobTemplateSource, owner domain.Object, reg *types.PresetRegistry) cronjobs.Spec {
	ownerName := owner.GetName()
	spec := cronjobs.Spec{
		Name:               src.Name,
		Namespace:          src.Namespace,
		Schedule:           src.Schedule,
		Image:              src.Image,
		Command:            src.Command,
		Args:               src.Args,
		Labels:             make(map[string]string),
		Resources:          presetResources(src.Resources, reg.All()),
		SecurityContext:    presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:        presetPodSecurity(src.PodSecurity, reg.All()),
		ImagePullSecrets:   src.ImagePullSecrets,
		NodeSelector:       src.NodeSelector,
		ServiceAccountName: src.ServiceAccountName,
		RestartPolicy:      corev1.RestartPolicy(src.RestartPolicy),
		Containers:         containerSpecs(src.Containers, reg.All()),
		InitContainers:     containerSpecs(src.InitContainers, reg.All()),
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
		Containers:         containerSpecs(src.Containers, reg.All()),
		InitContainers:     containerSpecs(src.InitContainers, reg.All()),
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

	spec.Protocol = ParseProtocol(src.Protocol)

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
		Name:               src.Name,
		Namespace:          src.Namespace,
		Image:              src.Image,
		Command:            src.Command,
		Args:               src.Args,
		BackoffLimit:       backoffLimit,
		Labels:             make(map[string]string),
		Resources:          presetResources(src.Resources, reg.All()),
		SecurityContext:    presetContainerSecurity(src.SecurityContext, reg.All()),
		PodSecurity:        presetPodSecurity(src.PodSecurity, reg.All()),
		Volumes:            src.Volumes,
		VolumeMounts:       src.VolumeMounts,
		ImagePullSecrets:   src.ImagePullSecrets,
		NodeSelector:       src.NodeSelector,
		ServiceAccountName: src.ServiceAccountName,
		RestartPolicy:      corev1.RestartPolicy(src.RestartPolicy),
		Containers:         containerSpecs(src.Containers, reg.All()),
		InitContainers:     containerSpecs(src.InitContainers, reg.All()),
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
	spec.Resources = presetResources(src.Resources, reg.All())
	spec.Probes = presetProbes(src.Probes, reg.All())
	spec.SecurityContext = presetContainerSecurity(src.SecurityContext, reg.All())
	spec.PodSecurity = presetPodSecurity(src.PodSecurity, reg.All())
	spec.Volumes = src.Volumes
	spec.VolumeMounts = src.VolumeMounts
	spec.ImagePullSecrets = src.ImagePullSecrets
	spec.NodeSelector = src.NodeSelector
	spec.ServiceAccountName = src.ServiceAccountName
	spec.RestartPolicy = corev1.RestartPolicy(src.RestartPolicy)
	spec.Containers = containerSpecs(src.Containers, reg.All())
	spec.InitContainers = containerSpecs(src.InitContainers, reg.All())

	if src.Port != "" {
		spec.Port = parsePort(src.Port)
	}
	spec.Protocol = ParseProtocol(src.Protocol)

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
		Containers:         containerSpecs(src.Containers, reg.All()),
		InitContainers:     containerSpecs(src.InitContainers, reg.All()),
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
	spec.Protocol = ParseProtocol(src.Protocol)

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	spec.Env = []types.EnvVar(src.Env)

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
		Containers:         containerSpecs(src.Containers, reg.All()),
		InitContainers:     containerSpecs(src.InitContainers, reg.All()),
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

	if p, err := strconv.ParseInt(src.Port, 10, 32); err == nil {
		spec.Port = int32(p)
	}
	spec.Protocol = ParseProtocol(src.Protocol)

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
