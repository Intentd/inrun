//go:build !runtime && !gateway

package simulate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer/crds"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"k8s.io/client-go/tools/cache"
)

const (
	envtestBinDir = ".inrun/envtest-bins"
	// DefaultEnvtestK8sVersion is the Kubernetes version used when --envtest-version
	// is not set. "1.32" (two components) resolves to the latest stable 1.32.x patch
	// from the controller-runtime release index. The setup-envtest "1.32.x" wildcard
	// syntax is not accepted by the Go API — two-component form is equivalent.
	DefaultEnvtestK8sVersion = "1.32"
)

// RunWithEnvtest is identical to Run but starts a local kube-apiserver + etcd,
// installs the CRDs at crdPaths, applies the CR, then runs the shared loop
// against a real SharedIndexInformer watching the envtest API server.
//
// crdPaths must contain at least one CRD YAML — set via spec.crd / spec.crdFiles.
// Binaries are auto-downloaded to ~/.inrun/envtest-bins on first run; KUBEBUILDER_ASSETS
// overrides this.
func RunWithEnvtest(ctx context.Context, bp *blueprint.Blueprint, crdName string,
	cr *unstructured.Unstructured, maxCycles int, opts RunOptions, crdPaths []string, k8sVersion string) (*Result, error) {
	defer quietLogs(opts)()
	defer stubExternal(opts)()

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

	restCfg, err := env.Start()
	if err != nil {
		return nil, fmt.Errorf("envtest: starting API server: %w", err)
	}
	defer env.Stop() //nolint:errcheck

	crdEntry, ok := bp.CRDEntry(crdName)
	if !ok {
		return nil, fmt.Errorf("CRD %q not found in Blueprint", crdName)
	}
	result := &Result{}
	if opts.Target != "" {
		// As if created through that target, so the reconcile resolves it.
		cr = cr.DeepCopy()
		intent.SetTarget(cr, opts.Target)
	}
	// Templates that need a live API server's cross-namespace reads are left
	// out of a copy of the entry; the notes say what was omitted and why.
	crdEntry, result.Notes = simulatable(crdEntry)

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
	setup, err := setupObjects(&crdEntry, opts.Setup)
	if err != nil {
		return nil, err
	}
	for _, obj := range setup {
		res, err := kubeclient.ResourceFor(recKube, obj)
		if err != nil {
			return nil, fmt.Errorf("setup %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
		if _, err := res.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			return nil, fmt.Errorf("setup %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	gvr := crdEntry.GVR()
	crData, err := json.Marshal(cr.Object)
	if err != nil {
		return nil, fmt.Errorf("marshalling CR: %w", err)
	}
	var crObj unstructured.Unstructured
	if err := json.Unmarshal(crData, &crObj.Object); err != nil {
		return nil, fmt.Errorf("copying CR: %w", err)
	}
	ns := crObj.GetNamespace()
	if ns == "" {
		_, err = recKube.DynamicClient().Resource(gvr).Create(ctx, &crObj, metav1.CreateOptions{})
	} else {
		_, err = recKube.DynamicClient().Resource(gvr).Namespace(ns).Create(ctx, &crObj, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("seeding CR into envtest: %w", err)
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
	inf := crds.Informer(ctx, crdEntry, realKube, infFactory, qr)
	infFactory.Start(ctx)

	// Observe secondary resources declared in operatorBox.observe.watch/events.
	// It starts its own informers and registers them with the factory.
	obs := observe.New(observe.Dependencies{
		Kube:          realKube,
		Informer:      infFactory,
		QueueRegistry: qr,
		Blueprint:     bp,
	})
	obs.Observe(ctx, crdEntry)

	// Wait after Observe, so cycle 1 reads the synced CR, watch caches and indexes.
	if !infFactory.WaitForCacheSync(ctx) {
		return nil, fmt.Errorf("envtest: informer cache sync timed out")
	}

	peerRegistry := coordinator.NewCoordinatorRegistry()
	for _, peerName := range bp.CRDNames() {
		if peerName == crdEntry.Name {
			continue
		}
		peerEntry, ok := bp.CRDEntry(peerName)
		if !ok {
			continue
		}
		peerCR, ok := opts.Peers[strings.ToLower(peerEntry.APITypes.Kind)]
		if !ok {
			continue
		}
		peerIdx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		_ = peerIdx.Add(peerCR)
		peerInf := newFakeInformer(peerIdx)
		peerGVKStr := schema.GroupVersionKind{
			Group:   peerEntry.APITypes.Group,
			Version: peerEntry.APITypes.Version,
			Kind:    peerEntry.APITypes.Kind,
		}.String()
		peerRegistry.Register(peerGVKStr, peerEntry, peerInf, nil)
	}

	// The reconciler the runtime would build for this CRD.
	recFactory, err := reconcilers.Factory(crdEntry, reconcilers.Deps{
		Kube:         recKube,
		Informer:     inf,
		Event:        event.Discard(),
		StoreFor:     infFactory.StoreFor,
		IndexerFor:   infFactory.IndexerFor,
		Blueprint:    bp,
		OwnNamespace: outsidePodNamespace,
	})
	if err != nil {
		return nil, err
	}
	r := recFactory()
	if crdEntry.WithRemoteDecl() || crdEntry.HasTargetRemoteDeclarations() {
		result.Notes = append(result.Notes, remoteNote)
	}

	key, err := resolveCacheKey(cr)
	if err != nil {
		return nil, err
	}

	deps := reconcile.Deps{
		Kube:      recKube,
		Event:     event.Discard(),
		Blueprint: bp,
		Registry:  peerRegistry,
	}
	input := reconcile.Input{
		GVK:        crdEntry.GVKString(),
		Key:        key,
		Entry:      contract.RegistryEntry{CRD: crdEntry, Informer: inf},
		Reconciler: r,
		Health:     vitals.NewCRDHealth(crdEntry.Name),
	}

	// The same pass the runtime runs.
	loopResult := runLoop(ctx, deps, input, recKube, maxCycles)
	loopResult.Notes = result.Notes
	return loopResult, nil
}

// ── HTTP transport recording ────────────────────────────────────────────────
//
// recordingTransport wraps the real HTTP transport and intercepts every
// successful write request to record an Op. This captures ALL kubeclient paths
// (typed clientset, dynamic client, controller-runtime client) without needing
// reactor chains, which only work on fake clients.

type recordingTransport struct {
	inner  http.RoundTripper
	shared *sharedOps
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A finalizer patch is told apart by its body: keep a copy before sending.
	var patch []byte
	if req.Method == http.MethodPatch && req.Body != nil {
		patch, _ = io.ReadAll(req.Body)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(patch))
	}

	resp, err := rt.inner.RoundTrip(req)
	if err != nil || resp == nil || !isWriteMethod(req.Method) || resp.StatusCode >= 300 {
		return resp, err
	}

	verb, resource, namespace, name := parseAPIRequest(req)
	resource = patchedPart(req, resource, patch)

	// POST creates: name is absent from the collection URL — read it from the
	// response body, then restore the body for the caller.
	if req.Method == http.MethodPost && name == "" {
		resource, namespace = parseCollectionURL(req)
		if resource != "" {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(body))
			if readErr == nil {
				name = extractMetaName(body)
				verb = "create"
			}
		}
	}

	if resource != "" && name != "" {
		rt.shared.mu.Lock()
		rt.shared.ops = append(rt.shared.ops, Op{
			Cycle:     rt.shared.cycle,
			Verb:      verb,
			Resource:  resource,
			Namespace: namespace,
			Name:      name,
			At:        time.Now(),
		})
		rt.shared.mu.Unlock()
	}
	return resp, err
}

// patchedPart names a patch the way the fake client records it, so one
// simulate.yaml asserts the same ops in both modes: a patch of the status
// subresource is "status", a patch of only metadata.finalizers is
// "finalizers". Any other write keeps its resource.
func patchedPart(req *http.Request, resource string, body []byte) string {
	if req.Method != http.MethodPatch {
		return resource
	}
	if strings.HasSuffix(req.URL.Path, "/status") {
		return "status"
	}
	var patch map[string]map[string]json.RawMessage
	if json.Unmarshal(body, &patch) == nil && len(patch) == 1 && len(patch["metadata"]) == 1 {
		if _, ok := patch["metadata"]["finalizers"]; ok {
			return "finalizers"
		}
	}
	return resource
}

func isWriteMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

// parseAPIRequest extracts (verb, resource, namespace, name) from a Kubernetes
// API request. Returns empty strings for requests that don't target a named
// object (e.g. list, watch, collection-level POST) or for subresource paths
// we intentionally skip.
//
// The cases are ordered so that a namespaced URL cannot be mistaken for a
// cluster-scoped one: namespaced branches require the "namespaces" segment at
// the expected position, and the cluster-scoped fallbacks are bounded so they
// cannot match a longer namespaced path.
//
// Supported URL formats:
//
//	/api/v1/namespaces/{ns}/{resource}/{name}[/{subresource}]
//	/api/v1/{resource}/{name}[/{subresource}]
//	/apis/{group}/{version}/namespaces/{ns}/{resource}/{name}[/{subresource}]
//	/apis/{group}/{version}/{resource}/{name}[/{subresource}]
func parseAPIRequest(req *http.Request) (verb, resource, namespace, name string) {
	p := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.Split(p, "/")

	var res, ns, n string
	switch {
	// Core group, namespaced:
	//   /api/v1/namespaces/{ns}/{resource}[/{name}][/subresource]
	case len(parts) >= 5 && parts[0] == "api" && parts[2] == "namespaces":
		ns, res = parts[3], parts[4]
		if len(parts) >= 6 {
			n = parts[5]
		}

	// Core group, cluster-scoped:
	//   /api/v1/{resource}[/{name}][/subresource]
	// Bounded so it cannot match a longer namespaced path; namespaced paths
	// always have at least 5 segments with parts[2] == "namespaces" and are
	// handled by the case above.
	case len(parts) >= 3 && len(parts) < 5 && parts[0] == "api":
		res = parts[2]
		if len(parts) >= 4 {
			n = parts[3]
		}

	// Non-core, namespaced:
	//   /apis/{group}/{version}/namespaces/{ns}/{resource}[/{name}][/subresource]
	case len(parts) >= 6 && parts[0] == "apis" && parts[3] == "namespaces":
		ns, res = parts[4], parts[5]
		if len(parts) >= 7 {
			n = parts[6]
		}

	// Non-core, cluster-scoped:
	//   /apis/{group}/{version}/{resource}[/{name}][/subresource]
	// Bounded so it cannot match a namespaced path.
	case len(parts) >= 4 && len(parts) < 6 && parts[0] == "apis":
		res = parts[3]
		if len(parts) >= 5 {
			n = parts[4]
		}
	}

	if n == "" || strings.Contains(n, "?") {
		return
	}

	// Map HTTP method + content type to Op verb.
	ct := req.Header.Get("Content-Type")
	switch req.Method {
	case http.MethodPost:
		verb = "create"
	case http.MethodPut:
		verb = "update"
	case http.MethodDelete:
		verb = "delete"
	case http.MethodPatch:
		if strings.Contains(ct, "apply-patch") {
			verb = "apply"
		} else {
			verb = "patch"
		}
	}

	resource, namespace, name = res, ns, n
	return
}

// ── loopKube adapter ─────────────────────────────────────────────────────────

type sharedOps struct {
	mu    sync.Mutex
	ops   []Op
	cycle int
}

// managedKube adapts a real kubeclient.Interface to the loopKube interface by
// wiring AdvanceCycle / OpsForCycle / Ops / MarkDeploymentReady to the shared op
// store, which is populated by the recording HTTP transport.
type managedKube struct {
	kubeclient.Interface
	shared *sharedOps
}

func (m *managedKube) AdvanceCycle() {
	m.shared.mu.Lock()
	m.shared.cycle++
	m.shared.mu.Unlock()
}

func (m *managedKube) OpsForCycle(cycle int) []Op {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	var out []Op
	for _, op := range m.shared.ops {
		if op.Cycle == cycle {
			out = append(out, op)
		}
	}
	return out
}

func (m *managedKube) Ops() []Op {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	out := make([]Op, len(m.shared.ops))
	copy(out, m.shared.ops)
	return out
}

func (m *managedKube) MarkDeploymentReady(namespace, name string) {
	dep, err := m.Interface.Clientset().AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return
	}
	if dep.Spec.Replicas != nil {
		dep.Status.AvailableReplicas = *dep.Spec.Replicas
		dep.Status.ReadyReplicas = *dep.Spec.Replicas
		dep.Status.Replicas = *dep.Spec.Replicas
	}
	m.Interface.Clientset().AppsV1().Deployments(namespace).UpdateStatus( //nolint:errcheck
		context.Background(), dep, metav1.UpdateOptions{})
}

// compile-time check.
var _ loopKube = (*managedKube)(nil)
