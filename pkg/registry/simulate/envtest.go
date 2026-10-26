//go:build !runtime && !gateway

package simulate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

const (
	envtestBinDir = ".inrun/envtest-bins"
	// DefaultEnvtestK8sVersion is the Kubernetes version used when --envtest-version
	// is not set. "1.32" (two components) resolves to the latest stable 1.32.x patch
	// from the controller-runtime release index. The setup-envtest "1.32.x" wildcard
	// syntax is not accepted by the Go API — two-component form is equivalent.
	DefaultEnvtestK8sVersion = "1.32"
)

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
	resp, err := rt.inner.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode >= 300 {
		return resp, err
	}
	w, ok := describeWrite(req)
	if !ok {
		return resp, err
	}

	// A create to a collection carries no name in its path: read it from
	// the response body, then restore the body for the caller.
	if w.name == "" && w.verb == "create" {
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		if readErr == nil {
			w.name = extractMetaName(body)
		}
	}
	if w.name == "" {
		return resp, err
	}

	rt.shared.mu.Lock()
	rt.shared.ops = append(rt.shared.ops, Op{
		Cycle:     rt.shared.cycle,
		Verb:      w.verb,
		Group:     w.group,
		Version:   w.version,
		Resource:  patchedPart(req, w.resource),
		Namespace: w.namespace,
		Name:      w.name,
		At:        time.Now(),
	})
	rt.shared.mu.Unlock()
	return resp, err
}

// patchedPart names a write the way the fake client records it, so one
// simulate.yaml asserts the same ops in both modes. The fake client names a
// write by the kubeclient method that made it — PatchFinalizers,
// PatchLabels, PatchAnnotations, PatchStatus, PatchSpec — and those methods
// tag the request context with that name. Any other write keeps its
// resource, as the fake client's reactors record it, even when its body
// touches only labels, and even when it writes a subresource.
func patchedPart(req *http.Request, resource string) string {
	if part := kubeclient.PatchPartFrom(req.Context()); part != "" {
		return part
	}
	return resource
}

// requestInfo parses a request path the way the API server does.
var requestInfo = &request.RequestInfoFactory{
	APIPrefixes:          sets.NewString("api", "apis"),
	GrouplessAPIPrefixes: sets.NewString("api"),
}

// apiWrite is one write request to the API, as the server reads its path.
type apiWrite struct {
	verb, group, version, resource, namespace, name string
}

// describeWrite reads a request the way the API server routes it — group,
// version, namespace, resource and name, cluster-scoped or namespaced, with
// or without a subresource — and reports whether it is a write to one
// object: create, update, patch, apply or delete. A server-side apply is a
// patch sent as apply-patch. A create to a collection has no name yet; reads
// and deletes of a whole collection are not writes to one object.
func describeWrite(req *http.Request) (apiWrite, bool) {
	info, err := requestInfo.NewRequestInfo(req)
	if err != nil || !info.IsResourceRequest {
		return apiWrite{}, false
	}
	w := apiWrite{
		verb:      info.Verb,
		group:     info.APIGroup,
		version:   info.APIVersion,
		resource:  info.Resource,
		namespace: info.Namespace,
		name:      info.Name,
	}
	// The server takes a Namespace to be in itself, for authorization; it
	// is a cluster-scoped object, and the fake records it so.
	if w.group == "" && w.resource == "namespaces" {
		w.namespace = ""
	}
	switch info.Verb {
	case "create", "update", "delete":
	case "patch":
		if strings.Contains(req.Header.Get("Content-Type"), "apply-patch") {
			w.verb = "apply"
		}
	default:
		return apiWrite{}, false
	}
	return w, true
}

// ── loopKube adapter ─────────────────────────────────────────────────────────

type sharedOps struct {
	mu    sync.Mutex
	ops   Ops
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

func (m *managedKube) OpsForCycle(cycle int) Ops {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	var out Ops
	for _, op := range m.shared.ops {
		if op.Cycle == cycle {
			out = append(out, op)
		}
	}
	return out
}

func (m *managedKube) Ops() Ops {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	out := make(Ops, len(m.shared.ops))
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

// Object reads one object back from the API server, as it stands after the
// simulation: with the server's defaulting and pruning applied.
func (m *managedKube) Object(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return m.Interface.DynamicClient().Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// catchUpTimeout bounds how long CatchUp waits for the informer.
const catchUpTimeout = 10 * time.Second

// CatchUp waits until the informer holds namespace/name at the version the
// API server holds now. The informer receives a write by watch, some time
// after it is made; a cycle that starts before then prepares from the CR as
// it was, and writes again what the last one wrote — a difference the
// in-memory run never shows. A deleted object has nothing to wait for.
func (m *managedKube) CatchUp(ctx context.Context, inf cache.SharedIndexInformer, gvr schema.GroupVersionResource, namespace, name string) error {
	if inf == nil {
		return nil
	}
	live, err := m.Object(ctx, gvr, namespace, name)
	if err != nil {
		return nil
	}
	want := live.GetResourceVersion()
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	// The informer was synced before cycle 1. If it holds no copy of the
	// object now, it never will — its namespace rules filter it out — so
	// there is nothing to catch up with.
	if _, exists, err := inf.GetIndexer().GetByKey(key); err != nil || !exists {
		return nil
	}
	err = wait.PollUntilContextTimeout(ctx, 20*time.Millisecond, catchUpTimeout, true, func(context.Context) (bool, error) {
		raw, exists, err := inf.GetIndexer().GetByKey(key)
		if err != nil || !exists {
			return false, nil
		}
		acc, err := meta.Accessor(raw)
		if err != nil {
			return false, nil
		}
		return atLeast(acc.GetResourceVersion(), want), nil
	})
	if err != nil {
		return fmt.Errorf("informer had not seen %s at resourceVersion %s after %s", key, want, catchUpTimeout)
	}
	return nil
}

// atLeast reports whether resourceVersion got is want or later. etcd's
// versions are increasing integers; any other pair is compared for equality.
func atLeast(got, want string) bool {
	g, gerr := strconv.ParseUint(got, 10, 64)
	w, werr := strconv.ParseUint(want, 10, 64)
	if gerr != nil || werr != nil {
		return got == want
	}
	return g >= w
}

// compile-time check.
var _ loopKube = (*managedKube)(nil)

// seedEnvtest SSA-applies setup objects and the CR to the API server before
// the simulation loop starts. Idempotent: re-runs overwrite stale state.
func seedEnvtest(
	ctx context.Context,
	kube kubeclient.Interface,
	crdEntry *types.CRDEntry,
	extraSetup []string,
	cr *unstructured.Unstructured,
) error {
	applyOpts := metav1.PatchOptions{FieldManager: "inrun-simulate", Force: ptr.To(true)}

	setup, err := setupObjects(crdEntry, extraSetup)
	if err != nil {
		return err
	}
	for _, obj := range setup {
		res, _, err := kubeclient.DynamicResourceFor(kube, obj.GroupVersionKind())
		if err != nil {
			return fmt.Errorf("setup %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("setup %s %s: marshal: %w", obj.GetKind(), obj.GetName(), err)
		}
		if _, err := kubeclient.PatchUnstructured(ctx, res, obj, k8stypes.ApplyPatchType, data, applyOpts); err != nil {
			return fmt.Errorf("setup %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	crObj := cr.DeepCopy()
	crData, err := json.Marshal(crObj)
	if err != nil {
		return fmt.Errorf("seeding CR into envtest: marshal: %w", err)
	}
	res := kube.DynamicClient().Resource(crdEntry.GVR())
	if _, err = kubeclient.PatchUnstructured(ctx, res, crObj, k8stypes.ApplyPatchType, crData, applyOpts); err != nil {
		return fmt.Errorf("seeding CR into envtest: %w", err)
	}
	return nil
}
