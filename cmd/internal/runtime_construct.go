// constructRuntime assembles the complete Inrun runtime registry.
//
// This is the dependency-injection boundary for the runtime: components are
// constructed here and their dependencies are wired here. Nothing is started
// here; startup is handled by inrun.Start() in declaration order.
//
// The resulting registry contains the Catalog, Kubernetes clients and
// informers, resource registries, Coordinator, and health server.
package internal

import (
	"context"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/process"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/generic"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/mux"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/tools/cache"
)

// runtimeConfig is the assembled runtime — returned to runtime.go so it can call
// inrun.Start(ctx) and block until shutdown.
type runtimeConfig struct {
	config  *config.Config
	catalog *catalog.Catalog
	komp    *[]domain.Component
	event   *event.Event
	kube    *kubeclient.Kubeclient
	cord    *coordinator.DependencyCoordinator
	inrun   *process.Manager
}

// constructRuntime wires the entire Inrun runtime.
//
// Nothing is started here. Every component is constructed and threaded together
// as closures and pointers. inrun.Start() calls component.Start() in
// registration order, and component.Stop() in reverse order on shutdown.
//
// The method is intentionally long — this is the one place where all wiring
// is visible. Splitting it would scatter the dependency graph across files
// and make it harder to reason about startup order.
func constructRuntime(cfg *config.Config, m *merger.Merger, ctx context.Context) *runtimeConfig {

	inrunNamespace := cfg.Cluster().Namespace()

	// ── 1a. Instance ────────────────────────────────────────────────────────────
	cfg.SetInstance(config.Runtime())

	// ── 1b. Catalog ────────────────────────────────────────────────────────────
	// Loads and validates the YAML Catalog. After this point, cat.Enabled()
	// returns only CRDs that passed schema validation and are not disabled.
	// Invalid CRDs are logged and excluded — they do not block the operator.
	cat := pipeline.NewCatalog(cfg, m)

	if registryURL := cfg.RegistryConfig().RegistryURL; registryURL != "" {
		m.SetRegistryURL(registryURL)
		logger.Info().Str("registry", registryURL).Msg("registry URL configured from INRUN_REGISTRY")
	}

	// ── 2. Scheme ─────────────────────────────────────────────────────────────
	// Each CRD type (e.g. *PipelineList) must be registered with the scheme so
	// the REST client knows how to decode API server responses. For dynamic CRDs
	// (unstructured mode), this is a no-op — they use the dynamic client.
	scheme, err := catalog.NewSchemeRegistry(cat)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build scheme registry")
	}

	// ── 3. Core components ────────────────────────────────────────────────────
	// Created here, started later by inrun in registration order.

	kube := kubeclient.NewKubeclient(cfg, scheme)

	// kubeclient is started immediately — the informer factory's missing-CRD
	// check needs the REST config during construction, before inrun.Start().
	if err := kube.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start kubeclient")
	}

	// Clientset is built inside Start; capture after Start so cs is never nil.
	cs := kube.Clientset()

	// HealthServer — HTTP-only (health, readiness, metrics, Catalog API routes).
	// Routes registered below before Start() binds the port.
	hs := health.NewHealthServer(cfg)

	// Event recorder — surfaces notable state changes to the Kubernetes event stream
	// (visible via kubectl describe). Shared by all controllers that emit events.
	ev := event.NewEvent(kube)

	// Default work queue — rate-limiting reconcile queue shared by controllers that
	// do not need a dedicated queue. Bounded to prevent runaway reconcile storms.
	defaultWq := queue.NewWorkqueue("default workqueue")

	// Queue registry — maps GVK strings to dedicated work queues. Controllers
	// register here so that cross-controller enqueues are dispatched correctly.
	queueRegistry := queue.NewQueueRegistry()

	// ── 4a. REST client provider ──────────────────────────────────────────────
	// Associates each CRD type with a constructor that builds a typed REST
	// client. The constructor is deferred — called on first informer use.
	// Dynamic CRDs skip this — they use the dynamic client directly.
	provider := kube.NewClientProvider()

	for _, crd := range cat.Enabled() {
		crd := crd
		if crd.IsDynamic() {
			continue
		}
		object, list := crd.GetRuntimeObjects()
		logger.Debug().Str("gvk", crd.GVKString()).Msg("registering CRD client provider")

		provider.Register(object, func(k *kubeclient.Kubeclient) (informer.GenericClient, error) {
			return k.NewClient(list, kubeclient.CRDInfo{
				Kind:          crd.APITypes.Kind,
				Group:         crd.APITypes.Group,
				Version:       crd.APITypes.Version,
				APIPath:       crd.APITypes.APIPath,
				GroupVersion:  crd.GroupVersion,
				Plural:        crd.APITypes.Plural,
				Namespace:     crd.Namespace,
				Namespaced:    crd.IsNamespaced(),
				ForceConflict: crd.ResolveForceConflict(),
			})
		})
	}

	// ── 4b. Shared informer factory ───────────────────────────────────────────
	// Creates one SharedIndexInformer per CRD. On Start(), each informer opens
	// a watch against the API server and populates its in-memory cache.
	// Watch events are routed into per-CRD workqueues via handleEvent.
	infFactory := informer.SharedInformerFactory(kube.RestConfig(), informer.FactoryOptions{
		Provider:      provider,
		Scheme:        scheme,
		DefaultWq:     defaultWq,
		Config:        cfg,
		Catalog:       cat,
		ClientSet:     cs,
		QueueRegistry: queueRegistry,
	})

	// ── 4d. Coordinator registry + per-CRD wiring ──────────────────────────────
	// controllerRegistry maps GVK → (CRDEntry, SharedIndexInformer, ReconcilerFactory).
	// It also implements reconciler.CatalogRegistry via GetInformerByName,
	// enabling cross-CRD observation with zero API server calls.
	controllerRegistry := coordinator.NewCoordinatorRegistry()

	// ── 4e. CRD health map ────────────────────────────────────────────────────
	// One CRDHealth per CRD — shared between the DependencyCoordinator
	// (which updates it on each reconcile) and the HTTP health routes
	// (which read it on each request). All three reference the same pointers.
	crdHealthMap := make(map[string]*vitals.CRDHealth)
	for _, crd := range cat.Enabled() {
		gvk := crd.GVKString()
		crdHealthMap[gvk] = vitals.NewCRDHealth(crd.Name)
	}

	logger.Debug().Msg("wiring CRDs into coordinator registry...")

	for _, crd := range cat.Enabled() {
		crd := crd
		gvk := crd.GVKString()
		object, _ := crd.GetRuntimeObjects()

		wq := queueRegistry.Register(gvk, crd.QueueConfig())

		// compute selectors
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
			infFactory.RegisterNamespaceFilter(gvk, filter)
			logger.Debug().
				Str("crd", crd.APITypes.Kind).
				Str("filter", informer.NamespaceFilterSummary(filter)).
				Msg("informer: namespace filter registered (Tier 2)")
		}

		// Choose typed or dynamic informer.
		// Dynamic CRDs use *unstructured.Unstructured — no Go type needed.
		// Typed CRDs use the registered concrete Go type for type-safe access.
		var inf cache.SharedIndexInformer

		// For dynamic CRDs, use opts.Namespace from the Tier 1 filter when set;
		// otherwise fall back to crd.Namespace (operator-level namespace setting).
		dynNamespace := crd.Namespace
		if opts.Namespace != "" {
			dynNamespace = opts.Namespace
		}

		logger.Debug().
			Bool("dynamic:", crd.IsDynamic()).
			Msgf("[DEBUG] CRD %s: location = %q\n", crd.APITypes.Kind, crd.APITypes.Location)
		if crd.IsDynamic() {
			lw := kube.NewDynamicListerWatcher(kubeclient.CRDInfo{
				Kind:          crd.APITypes.Kind,
				Group:         crd.APITypes.Group,
				Version:       crd.APITypes.Version,
				APIPath:       crd.APITypes.APIPath,
				GroupVersion:  crd.GroupVersion,
				Plural:        crd.APITypes.Plural,
				Namespace:     dynNamespace,
				Namespaced:    crd.IsNamespaced(),
				ForceConflict: crd.ResolveForceConflict(),
			}, kubeclient.ListOptions{
				LabelSelector: labelSelector,
				FieldSelector: fieldSelector,
			})
			inf = infFactory.ForListerWatcher(lw, object, ctx, opts)
		} else {
			inf = infFactory.For(object, ctx, opts)
		}

		infCopy := inf

		// Build the reconciler factory.
		// For default: true CRDs — generic.Reconciler interprets the Catalog declaratively.
		// For default: false CRDs — a custom Constructor is required.
		//
		// The factory is a closure — it captures all values at construction time
		// and is called by startCRDWorkers after informers are synced.
		// Each call returns a fresh reconciler instance for one worker goroutine.
		var factory func() domain.Reconciler

		if crd.DefaultReconcile() {
			objCopy := object

			var anyHooks domain.AnyReconcileHooks
			if r := crd.Box().Reconcile; r != nil && r.HookFactory != nil {
				anyHooks = r.HookFactory()
			}

			logger.Debug().Str("gvk", gvk).Msg("wiring generic.Reconciler factory")

			// Hooks read reconcile.args with req.Args(), like every reconciler.
			var hookKube kubeclient.Interface = kube.
				WithForceConflict(crd.ResolveForceConflict())

			factory = func() domain.Reconciler {
				return generic.New(
					crd,
					infCopy,
					hookKube,
					anyHooks,
					func() domain.Object {
						return objCopy.DeepCopyObject().(domain.Object)
					},
					cat,
				)
			}
		} else if crd.WithRemoteDecl() {
			logger.Debug().Str("gvk", gvk).Msg("wiring RemoteReconciler factory")

			remoteDecl := crd.Box().Reconcile.Remote
			crdManagedResources := crd.ManagedResources()
			remoteKube := kube.WithStoreFor(infFactory.StoreFor)
			factory = func() domain.Reconciler {
				r, err := remote.New(remoteDecl, crd.GVK(), remoteKube, ev, crdManagedResources, inrunNamespace)
				if err != nil {
					logger.Fatal().Err(err).Str("gvk", gvk).Msg("failed to build remote reconciler")
				}
				return r
			}
		} else {
			if !crd.ConstructorEnabled() {
				logger.Fatal().
					Str("gvk", gvk).
					Msg("reconciler.default is false but no Constructor provided")
			}

			logger.Debug().Str("gvk", gvk).Msg("wiring custom reconciler factory")

			// Attach the informer and event recorder to a copy of the kube client.
			// Constructor authors access them via kube.GetInformer() etc.; args
			// arrive on each request: req.Args().
			var ctorKube kubeclient.Interface = kube.
				WithInformer(infCopy).
				WithEventRecorder(ev).
				WithStoreFor(infFactory.StoreFor).
				WithIndexerFor(infFactory.IndexerFor).
				WithForceConflict(crd.ResolveForceConflict())

			factory = func() domain.Reconciler {
				return crd.Box().Reconcile.Constructor(ctorKube)
			}
		}

		// Wrap with the mux reconciler when per-target constructors or remote reconcilers
		// are declared. The mux reconciler dispatches each reconcile cycle to the matching
		// target's domain.Reconciler, falling back to the base factory for CRs with
		// no annotation or an unrecognised target name.
		if crd.HasTargetConstructorFactories() || crd.HasTargetRemoteDeclarations() {
			baseFactory := factory
			crdCopy := crd
			factory = func() domain.Reconciler {
				ctorCount := len(crdCopy.TargetReconcilerFactories)
				remoteDecls := crdCopy.TargetRemoteDeclarations()
				targets := make(map[string]domain.Reconciler, ctorCount+len(remoteDecls))
				for targetName, ctor := range crdCopy.TargetReconcilerFactories {
					var targetKube kubeclient.Interface = kube.
						WithInformer(infCopy).
						WithEventRecorder(ev).
						WithStoreFor(infFactory.StoreFor).
						WithIndexerFor(infFactory.IndexerFor).
						WithForceConflict(crdCopy.ResolveForceConflict())
					targets[targetName] = ctor(targetKube)
				}
				for targetName, remoteDecl := range remoteDecls {
					r, err := remote.New(remoteDecl, crdCopy.GVK(), kube.WithStoreFor(infFactory.StoreFor), ev, crdCopy.ManagedResources(), inrunNamespace)
					if err != nil {
						logger.Fatal().Err(err).Str("gvk", gvk).Str("target", targetName).Msg("failed to build remote reconciler")
					}
					targets[targetName] = r
				}
				return mux.New(infCopy, targets, baseFactory())
			}
			logger.Debug().
				Str("gvk", gvk).
				Int("constructorTargets", len(crd.TargetReconcilerFactories)).
				Int("remoteTargets", len(crd.TargetRemoteDeclarations())).
				Msg("wiring mux reconciler factory")
		}

		// Register informs the DependencyCoordinator which informer and factory
		// belong to this CRD. Workers are not started yet — that happens in Start().
		controllerRegistry.Register(gvk, crd, inf, factory)
		logger.Debug().Str("gvk", gvk).Msg("CRD registered")
	}

	// ── 5. HTTP routes ───────────────────────────────────────────────────────
	// All routes registered before hs.Start() — the mux is shared.
	//
	// Per-CRD routes:
	//   /catalog/{crd}/health                 200 healthy, 503 degraded
	//   /catalog/{crd}                        CRD config and live reconcile stats
	//   /catalog/{crd}/raw                    the user's config
	//   /catalog/{crd}/enriched               the runtime config
	//   /catalog/{crd}/cr                     all CR instances (informer cache)
	//   /catalog/{crd}/cr/{ns}/{name}         CR detail and children
	//   /catalog/{crd}/cr/{ns}/{name}/events  recent events
	//
	// Aggregate:
	//   /catalog/raw                          the user's Catalog
	//   /catalog/enriched                     the runtime Catalog
	//   /catalog                              all CRDs, dependency graph, health summary
	inrunHealth := vitals.NewRuntimeHealth()

	for _, crd := range cat.Enabled() {
		gvk := crd.GVKString()
		crdHealth := crdHealthMap[gvk]
		crdName := strings.ToLower(crd.Name)

		entry, _ := controllerRegistry.Get(gvk)
		inf := entry.Informer

		if !crd.IsEnabledAllEndpoints() {
			continue
		}

		if crd.IsHealthEnabled() {
			hs.Register(
				"/catalog/"+crdName+"/health",
				vitals.BuildCRDHealthHandler(crd, cfg, inf, crdHealth, inrunHealth),
			)
		}

		if crd.IsInfoEnabled() {
			hs.Register(
				"/catalog/"+crdName,
				vitals.BuildCRDInfoHandler(
					crd, cat.RBACRulesForCRD(crd), cfg, inf, crdHealth,
					inrunHealth,
				),
			)
			hs.Register(
				"/catalog/"+crdName+"/cr",
				vitals.BuildCRListHandler(crd, inf, inrunHealth),
			)
			hs.Register(
				"/catalog/"+crdName+"/cr/",
				vitals.BuildCRDetailAndEventsHandler(crd, inf, kube, crd.Box(), inrunHealth),
			)
		}

		// Register raw and enriched CRD definition endpoint
		hs.Register(
			"/catalog/"+crdName+"/raw",
			vitals.BuildCRDRawHandler(m, crd.Name),
		)
		hs.Register(
			"/catalog/"+crdName+"/enriched",
			vitals.BuildCRDEnrichedHandler(cat, crd.Name),
		)

		logger.Debug().
			Str("health", "/catalog/"+crdName+"/health").
			Str("info", "/catalog/"+crdName).
			Str("raw", "/catalog/"+crdName+"/raw").
			Str("enriched", "/catalog/"+crdName+"/enriched").
			Msg("registered CRD routes")
	}

	hs.Register("/catalog/raw", vitals.BuildRawCatalogHandler(m))
	hs.Register("/catalog/enriched", vitals.BuildEnrichedCatalogHandler(cat))
	hs.Register("/catalog", vitals.BuildCatalogHandler(cat, cfg, controllerRegistry, crdHealthMap, inrunHealth))

	// ── 6a. Secondary resource observers ────────────────────────────
	// Observe secondary resources declared in operatorBox.observe.watch/events.
	obs := observe.New(observe.Dependencies{
		Kube:          kube,
		Informer:      infFactory,
		QueueRegistry: queueRegistry,
		Catalog:       cat,
	})

	// ── 6b. Dependency coordinator ──────────────────────────────────────────────
	// Starts CRD workers in topological order defined by the dependency graph.
	// For each CRD, waits until all declared dependsOn CRDs meet their
	// condition (started | healthy) before calling factory() and starting workers.
	//
	// Worker lifecycle:
	//   Start()  → wait for informer sync → call factory() per worker → run loop
	//   Shutdown → drain queue → stop workers → remove from active set
	coord := coordinator.NewDependencyCoordinator(
		kube,
		infFactory,
		obs,
		controllerRegistry,
		cat,
		ev,
		hs,
		queueRegistry,
		defaultWq,
		crdHealthMap,
		inrunHealth,
		cfg.Catalog().DefaultWorkers(),
		catalog.NewDependencyGraph(cat),
		cfg.Catalog().ShutdownTimeout(),
	)

	// ── 7. Component list ─────────────────────────────────────────────────────
	// Start order: each component must start after its dependencies.
	// Stop order: reverse of start order (automatic).
	//
	// HealthServer starts first so it can serve /ready during startup.
	// Kubeclient is already started above but is still registered so
	// inrun manages its Stop().
	components := []domain.Component{
		hs,            // 1. HTTP server — /ready, /livez, /catalog routes
		kube,          // 2. REST clients — already started, managed for Stop()
		ev,            // 3. event recorder — depends on kube
		queueRegistry, // 4. per-CRD bounded queues
		defaultWq,     // 5. default unbounded queue
		infFactory,    // 6. informer factory — starts watchers, closes ready channel
		coord,         // 7. dependency coordinator — starts workers in topo order
	}

	// ── 8. Inrun ───────────────────────────────────────────────────────────
	// The supervisor. Calls Start() on each component in order.
	// On OS signal (SIGTERM/SIGINT) or fatal error, calls Stop() in reverse.
	// Graceful shutdown: drains queues before stopping workers.
	o := process.New(
		cfg.RunningInstance(),
		cfg.Catalog().ShutdownGracePeriod(),
		cfg.Inrun().LogLevel(),
	)
	o.Register(components)

	return &runtimeConfig{
		config:  cfg,
		catalog: cat,
		komp:    &components,
		event:   ev,
		kube:    kube,
		cord:    coord,
		inrun:   o,
	}
}
