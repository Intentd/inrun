package generic

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GVR aliases used by reconciler-internal files (run_delete_ordered.go).
// All authoritative GVR definitions live in pkg/children.
var (
	deploymentGVR         = children.DeploymentGVR
	statefulSetGVR        = children.StatefulSetGVR
	replicaSetGVR         = children.ReplicaSetGVR
	serviceGVR            = children.ServiceGVR
	secretGVR             = children.SecretGVR
	configMapGVR          = children.ConfigMapGVR
	serviceAccountGVR     = children.ServiceAccountGVR
	jobGVR                = children.JobGVR
	cronJobGVR            = children.CronJobGVR
	ingressGVR            = children.IngressGVR
	pvcGVR                = children.PersistentVolumeClaimGVR
	pvGVR                 = children.PersistentVolumeGVR
	hpaGVR                = children.HorizontalPodAutoscalerGVR
	pdbGVR                = children.PodDisruptionBudgetGVR
	namespaceGVR          = children.NamespaceGVR
	podGVR                = children.PodGVR
	roleGVR               = children.RoleGVR
	roleBindingGVR        = children.RoleBindingGVR
	clusterRoleGVR        = children.ClusterRoleGVR
	clusterRoleBindingGVR = children.ClusterRoleBindingGVR
	networkPolicyGVR      = children.NetworkPolicyGVR
	limitRangeGVR         = children.LimitRangeGVR
	resourceQuotaGVR      = children.ResourceQuotaGVR
)

func runExternal(
	ctx context.Context,
	gvk string,
	resolver *template.Resolver,
	calls []types.ExternalCallSpec,
	cs kubernetes.Interface,
) (*template.Resolver, error) {
	return external.Run(ctx, gvk, resolver, calls, cs)
}

// hooksFor returns the ObjectHooks for the given target name.
// If the target has a distinct hook binary (registered in TargetHookFactories),
// those hooks are returned. Otherwise falls back to the CRD-level hooks.
func (r *Reconciler[PTR]) hooksFor(target string) domain.ObjectHooks {
	if target != "" {
		if h, ok := r.targetHooks[target]; ok {
			return h
		}
	}
	return r.hooks
}

// namespaceGuardFunc returns a guard closure pre-bound to this CRD's
// namespace restrictions, or nil when no restrictions are configured.
func (r *Reconciler[PTR]) namespaceGuardFunc() func(ctx context.Context, obj domain.Object, ns string) bool {
	restricted := r.crd.AllRestrictedNamespaces()
	allowed := r.crd.AllAllowedNamespaces()
	if len(restricted) == 0 && len(allowed) == 0 {
		return nil
	}
	kind := r.crd.APITypes.Kind
	return func(ctx context.Context, obj domain.Object, ns string) bool {
		return prepare.CheckNamespace(ctx, obj, ns, restricted, allowed, kind).Allowed
	}
}

// patchStripFinalizers removes all box-declared finalizers from obj and patches the API server.
func (r *Reconciler[PTR]) patchStripFinalizers(ctx context.Context, obj PTR, box types.OperatorBoxConfig) error {
	if len(obj.GetFinalizers()) == 0 {
		return nil
	}
	if !labels.StripFinalizers(obj, box.EffectiveFinalizers()) {
		return nil
	}
	logger.Debug().
		Str("name", obj.GetName()).
		Msgf("removing finalizers → %v", obj.GetFinalizers())
	return r.kube.PatchFinalizers(ctx, obj, obj.GetFinalizers(), metav1.PatchOptions{})
}
