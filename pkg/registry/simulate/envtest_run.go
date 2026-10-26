//go:build !windows

package simulate

// runWithEnvtest is built everywhere but Windows: controller-runtime's envtest
// (v0.25.2) does not compile for Windows — its internal process package
// declares signalProcess twice there. envtest_windows.go stands in for it.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/informer/crds"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func runWithEnvtest(ctx context.Context, opts RunOptions) (*Result, error) {
	defer quietLogs(opts)()
	defer stubExternal(opts)()

	bp := opts.Blueprint
	name := opts.CRDName
	cr := opts.CR
	maxCycles := opts.MaxCycles
	if maxCycles == 0 {
		maxCycles = 10
	}
	crdPaths := opts.CRDPaths
	k8sVersion := opts.K8sVersion

	binDir := os.Getenv("HOME") + "/" + envtestBinDir

	env := &envtest.Environment{
		BinaryAssetsDirectory: binDir,
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: true,
	}

	// Respect KUBEBUILDER_ASSETS; otherwise auto-download to ~/.inrun/envtest-bins.
	if assets := os.Getenv("KUBEBUILDER_ASSETS"); assets != "" {
		env.BinaryAssetsDirectory = assets
	} else {
		env.DownloadBinaryAssets = true
		if k8sVersion == "" {
			k8sVersion = DefaultEnvtestK8sVersion
		}
		env.DownloadBinaryAssetsVersion = k8sVersion
		// Only print the download notice when the binaries don't yet exist locally.
		if _, err := os.Stat(binDir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "envtest: downloading binaries to %s (one-time, subsequent runs use cache)\n", binDir)
		}
	}

	// Resolve the CRD before starting the API server: a wrong name fails
	// at once, not after the server's start-up.
	result, entry, cr, err := prepareCRDEntry(bp, cr, name, opts.Target)
	if err != nil {
		return nil, err
	}

	started := time.Now()
	restCfg, err := env.Start()
	if err != nil {
		return nil, fmt.Errorf("envtest: starting API server: %w", err)
	}
	defer env.Stop() //nolint:errcheck
	result.Startup = time.Since(started)

	scheme, err := bp.Scheme()
	if err != nil {
		return nil, fmt.Errorf("building scheme: %w", err)
	}

	// Attach the recording transport BEFORE building the kubeclient so that ALL
	// API calls (typed clientset, dynamic client, controller-runtime client) go
	// through the interceptor. The fake simulate path uses reactor chains; in
	// envtest there are no reactors, so transport-level interception is the only
	// reliable way to capture every write op regardless of which client is used.
	shared := &sharedOps{}
	restCfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return &recordingTransport{inner: rt, shared: shared}
	}

	realKube, err := kubeclient.NewFromConfig(ctx, restCfg, scheme)
	if err != nil {
		return nil, fmt.Errorf("building kubeclient: %w", err)
	}

	// managedKube wires the real kubeclient to the shared op store so that
	// loopKube methods (AdvanceCycle, OpsForCycle, etc.) work correctly.
	recKube := &managedKube{Interface: realKube, shared: shared}

	// Apply the CRD's setup objects (e.g. a remote reconciler's token Secret)
	// and the spec's, then the CR, to the real API server before the loop starts.
	if err := seedEnvtest(ctx, recKube, &entry, opts.Setup, cr); err != nil {
		return nil, err
	}

	// Discard the CR-seed op — it is infrastructure, not a reconcile output.
	shared.mu.Lock()
	shared.ops = shared.ops[:0]
	shared.mu.Unlock()

	// The CRD's informer and the shared factory, built as the runtime builds
	// them; the factory also holds the watches observe adds. Their queues are
	// never drained: each cycle runs the pass directly.
	qr := queue.NewQueueRegistry()
	infFactory := crds.NewFactory(config.NewDefaultConfig(), bp, realKube, scheme,
		queue.NewWorkqueue("default-harness-workqueue"), qr)
	inf := crds.Informer(ctx, entry, realKube, infFactory, qr)
	infFactory.Start(ctx)

	// Observe secondary resources declared in operatorBox.observe.watch/events.
	// It starts its own informers and registers them with the factory.
	obs := observe.New(observe.Dependencies{
		Kube:          realKube,
		Informer:      infFactory,
		QueueRegistry: qr,
		Blueprint:     bp,
	})
	obs.Observe(ctx, entry)

	// Wait after Observe, so cycle 1 reads the synced CR, watch caches and indexes.
	if !infFactory.WaitForCacheSync(ctx) {
		return nil, fmt.Errorf("envtest: informer cache sync timed out")
	}

	peerRegistry, err := buildPeerRegistry(bp, entry.Name, opts.Peers)
	if err != nil {
		return nil, err
	}

	return runReconcileLoop(ctx, entry, reconcileRun{
		recDeps: reconcilers.Deps{
			Kube:         recKube,
			Informer:     inf,
			Event:        event.Discard(),
			StoreFor:     infFactory.StoreFor,
			IndexerFor:   infFactory.IndexerFor,
			Blueprint:    bp,
			OwnNamespace: outsidePodNamespace,
		},
		reconcileKube: recKube,
		runKube:       recKube,
		informer:      inf,
		peerReg:       peerRegistry,
	}, bp, cr, result, maxCycles)
}
