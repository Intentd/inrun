// pkg/children/gvr.go
package children

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── Child resource GVRs ───────────────────────────────────────────────────
// These are the GVRs for every resource type the InrunRegistry creates.
// Used to read back child resources after reconcile completes.
// When you add a new resource, make it available here to be read by Inrun.

var (
	DeploymentGVR              = gvrOrPanic("deployment")
	ServiceGVR                 = gvrOrPanic("service")
	SecretGVR                  = gvrOrPanic("secret")
	ConfigMapGVR               = gvrOrPanic("configmap")
	JobGVR                     = gvrOrPanic("job")
	CronJobGVR                 = gvrOrPanic("cronjob")
	PodGVR                     = gvrOrPanic("pod")
	ServiceAccountGVR          = gvrOrPanic("serviceaccount")
	StatefulSetGVR             = gvrOrPanic("statefulset")
	IngressGVR                 = gvrOrPanic("ingress")
	PersistentVolumeClaimGVR   = gvrOrPanic("persistentvolumeclaim")
	PersistentVolumeGVR        = gvrOrPanic("persistentvolume")
	HorizontalPodAutoscalerGVR = gvrOrPanic("horizontalpodautoscaler")
	PodDisruptionBudgetGVR     = gvrOrPanic("poddisruptionbudget")
	DaemonSetGVR               = gvrOrPanic("daemonset")
	ReplicaSetGVR              = gvrOrPanic("replicaset")
	NetworkPolicyGVR           = gvrOrPanic("networkpolicy")
	ResourceQuotaGVR           = gvrOrPanic("resourcequota")
	LimitRangeGVR              = gvrOrPanic("limitrange")
	RoleGVR                    = gvrOrPanic("role")
	RoleBindingGVR             = gvrOrPanic("rolebinding")
	ClusterRoleGVR             = gvrOrPanic("clusterrole")
	ClusterRoleBindingGVR      = gvrOrPanic("clusterrolebinding")
	NamespaceGVR               = gvrOrPanic("namespace")
	NodeGVR                    = gvrOrPanic("node")
	EndpointSliceGVR           = gvrOrPanic("endpointslice")
	EventGVR                   = gvrOrPanic("event")
	StorageClassGVR            = gvrOrPanic("storageclass")
)

// gvrOrPanic is a small helper for static initialization.
// Built-ins are guaranteed to exist in blueprint.builtInRegistry.
func gvrOrPanic(kind string) schema.GroupVersionResource {
	gvr, ok := GVRForBuiltInByKind(kind)
	if !ok {
		panic("blueprint: built-in kind not found: " + kind)
	}
	return gvr
}

// ChildGVRs returns all built‑in child resource GVRs with their keys.
func ChildGVRs() []struct {
	GVR schema.GroupVersionResource
	Key string
} {
	return []struct {
		GVR schema.GroupVersionResource
		Key string
	}{
		{DeploymentGVR, "deployment"},
		{ServiceGVR, "service"},
		{ConfigMapGVR, "configmap"},
		{SecretGVR, "secret"},
		{JobGVR, "job"},
		{CronJobGVR, "cronjob"},
		{ServiceAccountGVR, "serviceaccount"},
		{StatefulSetGVR, "statefulset"},
		{IngressGVR, "ingress"},
		{PersistentVolumeClaimGVR, "persistentvolumeclaim"},
		{PersistentVolumeGVR, "persistentvolume"},
		{HorizontalPodAutoscalerGVR, "horizontalpodautoscaler"},
		{PodDisruptionBudgetGVR, "poddisruptionbudget"},
		{DaemonSetGVR, "daemonset"},
		{ReplicaSetGVR, "replicaset"},
		{NetworkPolicyGVR, "networkpolicy"},
		{ResourceQuotaGVR, "resourcequota"},
		{LimitRangeGVR, "limitrange"},
		{RoleGVR, "role"},
		{RoleBindingGVR, "rolebinding"},
		{ClusterRoleGVR, "clusterrole"},
		{ClusterRoleBindingGVR, "clusterrolebinding"},
		{NamespaceGVR, "namespace"},
		{NodeGVR, "node"},
	}
}
