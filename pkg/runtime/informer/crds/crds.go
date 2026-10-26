// Package crds builds the informers for a Blueprint's CRDs: the shared factory,
// with a typed REST client per compiled CRD, and each CRD's informer with its
// queue and namespace filter. The runtime and inrun simulate --envtest both
// use it, so a CRD is watched the same way in both.
package crds

import (
	"context"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// NewFactory builds the shared informer factory: one SharedIndexInformer per
// CRD. On Start(), each informer opens a watch against the API server and
// populates its in-memory cache; watch events are routed into per-CRD
// workqueues via handleEvent.
func NewFactory(
	cfg *config.Config,
	bp *blueprint.Blueprint,
	kube *kubeclient.Kubeclient,
	scheme *runtime.Scheme,
	defaultWq *queue.Workqueue,
	qr *queue.QueueRegistry,
) *informer.Factory {
	// Associates each CRD type with a constructor that builds a typed REST
	// client. The constructor is deferred — called on first informer use.
	// Dynamic CRDs skip this — they use the dynamic client directly.
	provider := NewClientProvider(kube)

	for _, crd := range bp.Enabled() {
		crd := crd
		if crd.IsDynamic() {
			continue
		}
		object, list, err := crd.RuntimeObjects()
		if err != nil {
			// Blueprint validation already reports a missing registration.
			logger.Error().Err(err).Str("gvk", crd.GVKString()).Msg("no client provider registered")
			continue
		}
		logger.Debug().Str("gvk", crd.GVKString()).Msg("registering CRD client provider")

		provider.Register(object, func(k *kubeclient.Kubeclient) (informer.GenericClient, error) {
			return k.NewClient(list, info(crd, crd.Namespace))
		})
	}

	// Clientset is built inside kube.Start, so kube must be started first.
	return informer.SharedInformerFactory(kube.RestConfig(), informer.FactoryOptions{
		Provider:      provider,
		Scheme:        scheme,
		DefaultWq:     defaultWq,
		Config:        cfg,
		Blueprint:     bp,
		ClientSet:     kube.Clientset(),
		QueueRegistry: qr,
	})
}

// Informer registers crd's queue and namespace filter and returns its
// informer, registered with f: typed for compiled CRDs, dynamic
// (*unstructured.Unstructured) otherwise.
func Informer(
	ctx context.Context,
	crd types.CRDEntry,
	kube *kubeclient.Kubeclient,
	f *informer.Factory,
	qr *queue.QueueRegistry,
) cache.SharedIndexInformer {
	gvk := crd.GVKString()
	object, _, _ := crd.RuntimeObjects() // validated with the Blueprint
	wq := qr.Register(gvk, crd.QueueConfig())

	labelSelector := types.Labels(crd.LabelSelector).String()
	fieldSelector := types.Labels(crd.FieldSelector).String()

	opts := informer.Options{
		Name:          crd.APITypes.Kind,
		Resync:        crd.SetResync(0),
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	if crd.SharedQueue() {
		opts.Wq = nil // use the shared default queue
	} else {
		opts.Wq = wq
	}

	// ── Namespace filter — Tier 1 (scope ListerWatcher) + Tier 2 (pre-enqueue) ──
	// Tier 2 is always registered when namespace rules exist.
	// Tier 1 scopes the ListerWatcher to a single namespace when allowedNamespaces
	// has exactly one entry — the informer never sees events from other namespaces.
	if crd.HasNamespaceRules() {
		if crd.IsSingleNamespace() {
			opts.Namespace = crd.SingleNamespace()
			logger.Debug().
				Str("crd", crd.APITypes.Kind).
				Str("namespace", opts.Namespace).
				Msg("informer: namespace-scoped watch (Tier 1)")
		}
		filter := &informer.NamespaceFilter{
			AllowedNamespaces:    []string(crd.AllAllowedNamespaces()),
			RestrictedNamespaces: []string(crd.AllRestrictedNamespaces()),
		}
		f.RegisterNamespaceFilter(gvk, filter)
		logger.Debug().
			Str("crd", crd.APITypes.Kind).
			Str("filter", informer.NamespaceFilterSummary(filter)).
			Msg("informer: namespace filter registered (Tier 2)")
	}

	if !crd.IsDynamic() {
		return f.For(object, ctx, opts)
	}

	// For dynamic CRDs, use opts.Namespace from the Tier 1 filter when set;
	// otherwise fall back to crd.Namespace (operator-level namespace setting).
	namespace := crd.Namespace
	if opts.Namespace != "" {
		namespace = opts.Namespace
	}
	lw := kube.NewDynamicListerWatcher(info(crd, namespace), kubeclient.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
	return f.ForListerWatcher(lw, object, ctx, opts)
}

// info describes crd to the kube client, watched in namespace.
func info(crd types.CRDEntry, namespace string) kubeclient.CRDInfo {
	return kubeclient.CRDInfo{
		Kind:          crd.APITypes.Kind,
		Group:         crd.APITypes.Group,
		Version:       crd.APITypes.Version,
		APIPath:       crd.APITypes.APIPath,
		GroupVersion:  crd.GroupVersion,
		Plural:        crd.APITypes.Plural,
		Namespace:     namespace,
		Namespaced:    crd.IsNamespaced(),
		ForceConflict: crd.ResolveForceConflict(),
	}
}
