package simulate

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
)

// New returns a Simulator bound to ctx and opts. Call Execute on the returned value.
func New(ctx context.Context, opts RunOptions) *Simulator {
	return &Simulator{ctx: ctx, opts: opts}
}

// Backend is where a simulation runs.
type Backend int

const (
	// InMemory runs against fake clients: fast, no cluster.
	InMemory Backend = iota
	// Envtest runs against a real kube-apiserver and etcd started by
	// envtest; opts.CRDPaths must name the CRDs to install.
	Envtest
)

func (b Backend) String() string {
	if b == Envtest {
		return "envtest"
	}
	return "in-memory"
}

// Execute runs the simulation on backend and returns its result, named and
// timed: Elapsed is the time spent reconciling, without the API server's
// start-up.
func (s *Simulator) Execute(backend Backend) (*Result, error) {
	start := time.Now()
	var (
		r   *Result
		err error
	)
	switch backend {
	case Envtest:
		if len(s.opts.CRDPaths) == 0 {
			return nil, fmt.Errorf("envtest requires spec.crd or spec.crdFiles to be set")
		}
		r, err = runWithEnvtest(s.ctx, s.opts)
	default:
		r, err = run(s.ctx, s.opts)
	}
	return finished(s.opts, start, r, err)
}

// finished names a run's result and records how long it reconciled: since
// start, without the API server's start-up.
func finished(opts RunOptions, start time.Time, r *Result, err error) (*Result, error) {
	if err != nil {
		return nil, err
	}
	r.Name = opts.CRDName
	r.Elapsed = time.Since(start) - r.Startup
	return r, nil
}

func run(ctx context.Context, opts RunOptions) (*Result, error) {
	defer quietLogs(opts)()
	defer stubExternal(opts)()

	bp := opts.Blueprint
	name := opts.CRDName
	cr := opts.CR
	maxCycles := opts.MaxCycles
	if maxCycles == 0 {
		maxCycles = 10
	}

	result, entry, cr, err := prepareCRDEntry(bp, cr, name, opts.Target)
	if err != nil {
		return nil, err
	}

	scheme, err := bp.Scheme()
	if err != nil {
		return nil, fmt.Errorf("building scheme: %w", err)
	}

	// The API server defaults every CRD-kind object from its schema and
	// knows each kind's scope; the fake takes both from the same CRD files
	// envtest installs. Without them the run still works, and says what it
	// could not do.
	crds := readCRDs(opts.CRDPaths)
	if note := crds.missing(entry.GVK()); note != "" {
		result.Notes = append(result.Notes, note)
	}
	fc, err := buildFakeCluster(scheme, cr, &entry, opts.ExistingInstances, opts.Setup, crds)
	if err != nil {
		return nil, err
	}
	fakeKube := fc.kube
	seedObj := fc.seedObj

	informer, err := newFakeInformer(seedObj)
	if err != nil {
		return nil, fmt.Errorf("building informer: %w", err)
	}

	peerRegistry, err := buildPeerRegistry(bp, entry.Name, opts.Peers)
	if err != nil {
		return nil, err
	}

	// The reconcile and the loop share the copy that holds the informer;
	// it shares the op log and the store with the base client.
	withInformer := fakeKube.WithInformer(informer).(*kubefake.Kubeclient)

	return runReconcileLoop(ctx, entry, reconcileRun{
		recDeps: reconcilers.Deps{
			Kube:         fakeKube,
			Informer:     informer,
			Event:        event.Discard(),
			Blueprint:    bp,
			OwnNamespace: outsidePodNamespace,
		},
		reconcileKube: withInformer,
		runKube:       withInformer,
		informer:      informer,
		peerReg:       peerRegistry,
	}, bp, cr, result, maxCycles)
}
